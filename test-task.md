# general
- ensure can login
- ensure can switch warehouse
- ensure can sigout

# Setting
## Unit Catalog
- ensure can create, edit and delete
- ensure prevent delete if unit still used in medicine

# Medicine
- ensure can create medicine
    - ensure price unit work properly and use unit catalog
- ensure can search sku, medicine name
- ensure can filter last stock opname date
- ensure in medicine list have last opname date
- ensure medicine can archive and unarchive

# Pos Cashier
- ensure warehouse scope properly
- ensure can search medicine by name, medicine code
- ensure can checkout
    - ensure after checkout is exist in order history, and check data is correct
- ensure stock decrease properly, and pricing properly
- ensure no race condition when checkout

# Order history
- ensure order can canceled
- ensure stock correct when canceled

# Inventory

## Stock opname
- ensure can create opname and finish properly
    - check in menu `medicine` that medicine included in opname have last opname in list and detail, and ensure it correct, and ensure stock is correct too
    - ensure if stockopname voided/canceled its not change medicine last stock opname

## batch
- ensure can filter by expiry
- ensure can filter by supplier
- ensure its warehouse scoped
- ensure can search by medicine
- ensure have link to detail restock


## Transfer
- ensure create transfer properly
- ensure its warehouse scoped
- ensure history is correct

# warehouse
- can create warehouse
- can edit warehouse properly
- can search code and name properly

# Cashier Role
- ensure cashier can open order history without error
