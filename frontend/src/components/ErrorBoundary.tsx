import { Component, type ReactNode } from "react";
import { Box, Button, Heading, Stack, Text } from "@chakra-ui/react";

type Props = { children: ReactNode };
type State = { error: Error | null };

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: unknown) {
    console.error("ErrorBoundary caught:", error, info);
  }

  reload = () => {
    window.location.reload();
  };

  render() {
    if (this.state.error) {
      return (
        <Box p={8} maxW="md" mx="auto" mt={20}>
          <Stack gap={4}>
            <Heading size="lg">Something went wrong</Heading>
            <Text color="fg.muted">{this.state.error.message}</Text>
            <Button onClick={this.reload} colorPalette="blue" alignSelf="flex-start">
              Reload
            </Button>
          </Stack>
        </Box>
      );
    }
    return this.props.children;
  }
}
