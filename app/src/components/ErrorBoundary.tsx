import { Component, ErrorInfo, ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface State {
  hasError: boolean;
  error?: Error;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("ErrorBoundary caught an error:", error, errorInfo);

    // Store error details for debugging
    if (typeof window !== "undefined") {
      const timestamp = new Date().toISOString();
      const errors = JSON.parse(localStorage.getItem("react_errors") || "[]");
      errors.push({
        timestamp,
        error: error.message,
        stack: error.stack,
        componentStack: errorInfo.componentStack,
      });
      // Keep only last 5 errors
      localStorage.setItem("react_errors", JSON.stringify(errors.slice(-5)));
    }

    this.props.onError?.(error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="flex items-center justify-center p-8 bg-red-900/20 border border-red-500/20 rounded-2xl">
          <div className="text-center">
            <h3 className="text-lg font-semibold text-red-300 mb-2">Something went wrong</h3>
            <p className="text-red-400 text-sm mb-4">
              This component encountered an error and could not render properly.
            </p>
            <button
              className="px-4 py-2 bg-red-900/40 border border-red-500/40 text-red-200 rounded-lg hover:bg-red-900/60 transition-colors"
              onClick={() => window.location.reload()}
            >
              Reload Page
            </button>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

// Hook-based error boundary for functional components
export function withErrorBoundary<P extends object>(
  Component: React.ComponentType<P>,
  fallback?: ReactNode
) {
  return function ComponentWithErrorBoundary(props: P) {
    return (
      <ErrorBoundary fallback={fallback}>
        <Component {...props} />
      </ErrorBoundary>
    );
  };
}
