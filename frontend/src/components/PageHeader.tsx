import { Box, Flex, HStack, Heading, Stack, Text } from "@chakra-ui/react";
import { ChevronRight } from "lucide-react";
import { Link as RouterLink } from "react-router-dom";
import type { ReactNode } from "react";

type Crumb = { label: string; to?: string };

type Props = {
  breadcrumbs?: Crumb[];
  title: string;
  description?: string;
  actions?: ReactNode;
};

export default function PageHeader({ breadcrumbs, title, description, actions }: Props) {
  return (
    <Stack gap={3} pb={4} mb={4} borderBottomWidth="1px">
      {breadcrumbs && breadcrumbs.length > 0 && (
        <HStack gap={1} color="fg.muted" fontSize="xs">
          {breadcrumbs.map((c, i) => (
            <HStack key={i} gap={1}>
              {c.to ? (
                <RouterLink to={c.to} style={{ textDecoration: "none" }}>
                  <Text _hover={{ color: "fg" }}>{c.label}</Text>
                </RouterLink>
              ) : (
                <Text>{c.label}</Text>
              )}
              {i < breadcrumbs.length - 1 && <ChevronRight size={12} />}
            </HStack>
          ))}
        </HStack>
      )}
      <Flex align="center" justify="space-between" gap={4}>
        <Box>
          <Heading size="xl">{title}</Heading>
          {description && (
            <Text fontSize="sm" color="fg.muted" mt={1}>
              {description}
            </Text>
          )}
        </Box>
        {actions && <HStack gap={2}>{actions}</HStack>}
      </Flex>
    </Stack>
  );
}
