package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	taxifacev1 "github.com/apotech/backend/gen/tax_iface/v1"
	"github.com/apotech/backend/internal/auth"
	"github.com/apotech/backend/internal/model"
	"github.com/apotech/backend/internal/sqldialect"
)

// PPN (VAT) rate as of 2026 in Indonesia. Hard-coded because it changes
// rarely; promote to config when it does.
const ppnNumerator, ppnDenominator = 11, 111

// TaxInvoice transaction code. "01" = standard PKP sale (default).
const defaultTaxInvoiceCode = "01"

type TaxInvoices struct {
	db *gorm.DB
}

func NewTaxInvoices(db *gorm.DB) *TaxInvoices { return &TaxInvoices{db: db} }

// ImportNsfpRange seeds the pool with [start_code .. end_code] inclusive. The
// NSFP body is the 13-char "XXX.YY.NNNNNNNN" portion DJP issues per fiscal
// year. We parse the trailing 8-digit serial, iterate, and insert with
// `ON CONFLICT DO NOTHING` so reimports are safe.
func (t *TaxInvoices) ImportNsfpRange(
	ctx context.Context,
	req *connect.Request[taxifacev1.ImportNsfpRangeRequest],
) (*connect.Response[taxifacev1.ImportNsfpRangeResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	startPrefix, startSeq, err := parseNsfpCode(req.Msg.StartCode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("start_code: %w", err))
	}
	endPrefix, endSeq, err := parseNsfpCode(req.Msg.EndCode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("end_code: %w", err))
	}
	if startPrefix != endPrefix {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("start_code and end_code must share the same XXX.YY prefix"))
	}
	if endSeq < startSeq {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("end_code must be >= start_code"))
	}
	if req.Msg.FiscalYear < 2000 || req.Msg.FiscalYear > 2100 {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("fiscal_year out of range"))
	}

	now := time.Now()
	imported, skipped := int32(0), int32(0)
	err = t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for seq := startSeq; seq <= endSeq; seq++ {
			row := model.NsfpEntry{
				Code:       fmt.Sprintf("%s.%08d", startPrefix, seq),
				FiscalYear: int(req.Msg.FiscalYear),
				ImportedBy: caller.UserID,
				ImportedAt: now,
			}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if res.Error != nil {
				return connect.NewError(connect.CodeInternal, res.Error)
			}
			if res.RowsAffected > 0 {
				imported++
			} else {
				skipped++
			}
		}
		return nil
	})
	if err != nil {
		return nil, asConnectErr(err)
	}
	return connect.NewResponse(&taxifacev1.ImportNsfpRangeResponse{
		ImportedCount: imported,
		SkippedCount:  skipped,
	}), nil
}

func (t *TaxInvoices) ListNsfp(
	ctx context.Context,
	req *connect.Request[taxifacev1.ListNsfpRequest],
) (*connect.Response[taxifacev1.ListNsfpResponse], error) {
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		if req.Msg.FiscalYear > 0 {
			q = q.Where("fiscal_year = ?", req.Msg.FiscalYear)
		}
		if req.Msg.UnusedOnly {
			q = q.Where("used_at IS NULL")
		}
		return q
	}

	var total int64
	if err := applyFilters(t.db.WithContext(ctx).Model(&model.NsfpEntry{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rows []model.NsfpEntry
	if err := applyFilters(t.db.WithContext(ctx).Model(&model.NsfpEntry{})).
		Order("code ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Also compute unused total (across all years or filtered year).
	countQ := t.db.WithContext(ctx).Model(&model.NsfpEntry{}).Where("used_at IS NULL")
	if req.Msg.FiscalYear > 0 {
		countQ = countQ.Where("fiscal_year = ?", req.Msg.FiscalYear)
	}
	var unusedTotal int64
	if err := countQ.Count(&unusedTotal).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*taxifacev1.NsfpEntry, 0, len(rows))
	for i := range rows {
		out = append(out, nsfpToProto(&rows[i]))
	}
	return connect.NewResponse(&taxifacev1.ListNsfpResponse{
		Entries:     out,
		UnusedTotal: int32(unusedTotal),
		Total:       int32(total),
	}), nil
}

func (t *TaxInvoices) ListTaxInvoices(
	ctx context.Context,
	req *connect.Request[taxifacev1.ListTaxInvoicesRequest],
) (*connect.Response[taxifacev1.ListTaxInvoicesResponse], error) {
	limit, offset := normPage(req.Msg.Limit, req.Msg.Offset)
	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Where("tax_invoice_no IS NOT NULL AND tax_invoice_no <> ''")
		if req.Msg.FromUnix > 0 {
			q = q.Where("tax_invoice_issued_at >= ?", time.Unix(req.Msg.FromUnix, 0))
		}
		if req.Msg.ToUnix > 0 {
			q = q.Where("tax_invoice_issued_at < ?", time.Unix(req.Msg.ToUnix, 0))
		}
		return q
	}

	var total int64
	if err := applyFilters(t.db.WithContext(ctx).Model(&model.Sale{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var sales []model.Sale
	if err := applyFilters(t.db.WithContext(ctx).Model(&model.Sale{})).
		Order("tax_invoice_issued_at DESC").Offset(offset).Limit(limit).Find(&sales).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(sales) == 0 {
		return connect.NewResponse(&taxifacev1.ListTaxInvoicesResponse{Total: int32(total)}), nil
	}
	custIDs := make([]string, 0, len(sales))
	for _, s := range sales {
		if s.CustomerID != nil {
			custIDs = append(custIDs, *s.CustomerID)
		}
	}
	custByID := map[string]model.Customer{}
	if len(custIDs) > 0 {
		var custs []model.Customer
		if err := t.db.WithContext(ctx).Where("id IN ?", custIDs).Find(&custs).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range custs {
			custByID[c.ID] = c
		}
	}
	out := make([]*taxifacev1.TaxInvoice, 0, len(sales))
	for i := range sales {
		var cust *model.Customer
		if sales[i].CustomerID != nil {
			if c, ok := custByID[*sales[i].CustomerID]; ok {
				cust = &c
			}
		}
		out = append(out, taxInvoiceToProto(&sales[i], cust))
	}
	return connect.NewResponse(&taxifacev1.ListTaxInvoicesResponse{
		Invoices: out,
		Total:    int32(total),
	}), nil
}

func (t *TaxInvoices) GetTaxInvoice(
	ctx context.Context,
	req *connect.Request[taxifacev1.GetTaxInvoiceRequest],
) (*connect.Response[taxifacev1.GetTaxInvoiceResponse], error) {
	var sale model.Sale
	err := t.db.WithContext(ctx).Where("id = ?", req.Msg.SaleId).First(&sale).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("sale not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sale.TaxInvoiceNo == nil || *sale.TaxInvoiceNo == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("sale has no tax invoice assigned"))
	}
	var cust *model.Customer
	if sale.CustomerID != nil {
		var c model.Customer
		if err := t.db.WithContext(ctx).Where("id = ?", *sale.CustomerID).First(&c).Error; err == nil {
			cust = &c
		}
	}
	return connect.NewResponse(&taxifacev1.GetTaxInvoiceResponse{
		Invoice: taxInvoiceToProto(&sale, cust),
	}), nil
}

// ---------- helpers ----------

// parseNsfpCode parses "XXX.YY.NNNNNNNN" and returns the prefix "XXX.YY" plus
// the integer NNNNNNNN serial.
func parseNsfpCode(code string) (prefix string, seq int64, err error) {
	code = strings.TrimSpace(code)
	parts := strings.Split(code, ".")
	if len(parts) != 3 {
		return "", 0, errors.New("NSFP code must be XXX.YY.NNNNNNNN")
	}
	if len(parts[0]) != 3 || len(parts[1]) != 2 || len(parts[2]) != 8 {
		return "", 0, errors.New("NSFP code parts must be 3.2.8 digits")
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return "", 0, errors.New("NSFP code parts must be numeric")
		}
	}
	seq, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", 0, err
	}
	return parts[0] + "." + parts[1], seq, nil
}

// AssignTaxInvoiceForSaleTx consumes the lowest-unused NSFP for the given
// fiscal year and assigns it to the sale, computing DPP/PPN. Caller owns the
// tx. The sale must already be COMPLETED.
func AssignTaxInvoiceForSaleTx(tx *gorm.DB, sale *model.Sale) error {
	if sale.TaxInvoiceNo != nil && *sale.TaxInvoiceNo != "" {
		return nil // already assigned, idempotent
	}
	year := time.Now().Year()
	if sale.CompletedAt != nil {
		year = sale.CompletedAt.Year()
	}

	var nsfp model.NsfpEntry
	err := tx.Where("fiscal_year = ? AND used_at IS NULL", year).
		Order("code ASC").
		Clauses(sqldialect.LockForUpdateSkipLocked()).
		First(&nsfp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("no NSFP available for fiscal year %d; import a range first", year))
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	now := time.Now()
	code := defaultTaxInvoiceCode
	// Standard format: CCC.SSS-YY.NNNNNNNN — but DJP stopped requiring SSS in
	// 2024; we keep the original NSFP body and prepend the 3-char code.
	taxNo := fmt.Sprintf("%s.%s", code, nsfp.Code)
	// DPP = total / 1.PPN%; PPN = total - DPP. We treat `total` as VAT-inclusive
	// (the common Indonesian apotek convention).
	dpp := sale.Total * int64(ppnDenominator-ppnNumerator) / int64(ppnDenominator)
	ppn := sale.Total - dpp

	// Mark NSFP used.
	if err := tx.Model(&nsfp).Updates(map[string]any{
		"used_at": now,
		"sale_id": sale.ID,
	}).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Stamp sale.
	updates := map[string]any{
		"tax_invoice_no":        taxNo,
		"tax_invoice_code":      code,
		"tax_invoice_dpp":       dpp,
		"tax_invoice_ppn":       ppn,
		"tax_invoice_issued_at": now,
	}
	if err := tx.Model(sale).Updates(updates).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	sale.TaxInvoiceNo = &taxNo
	sale.TaxInvoiceCode = &code
	sale.TaxInvoiceDPP = dpp
	sale.TaxInvoicePPN = ppn
	sale.TaxInvoiceIssuedAt = &now
	return nil
}

// ---------- proto mapping ----------

func nsfpToProto(n *model.NsfpEntry) *taxifacev1.NsfpEntry {
	out := &taxifacev1.NsfpEntry{
		Id:         n.ID,
		Code:       n.Code,
		FiscalYear: int32(n.FiscalYear),
		ImportedBy: n.ImportedBy,
		ImportedAt: n.ImportedAt.Unix(),
	}
	if n.UsedAt != nil {
		out.UsedAt = n.UsedAt.Unix()
	}
	if n.SaleID != nil {
		out.SaleId = *n.SaleID
	}
	return out
}

func taxInvoiceToProto(s *model.Sale, c *model.Customer) *taxifacev1.TaxInvoice {
	out := &taxifacev1.TaxInvoice{
		SaleId: s.ID,
		Dpp:    s.TaxInvoiceDPP,
		Ppn:    s.TaxInvoicePPN,
		Total:  s.TaxInvoiceDPP + s.TaxInvoicePPN,
	}
	if s.SaleNo != nil {
		out.SaleNo = *s.SaleNo
	}
	if s.TaxInvoiceNo != nil {
		out.TaxInvoiceNo = *s.TaxInvoiceNo
	}
	if s.TaxInvoiceCode != nil {
		out.TaxInvoiceCode = *s.TaxInvoiceCode
	}
	if s.TaxInvoiceIssuedAt != nil {
		out.IssuedAt = s.TaxInvoiceIssuedAt.Unix()
	}
	if c != nil {
		out.CustomerName = c.Name
		out.CustomerNpwp = c.NPWP
		out.CustomerAddress = c.Address
	}
	return out
}
