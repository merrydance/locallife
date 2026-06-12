function unwrapApiErrorMessage(message: string): string {
  const apiErrorMatch = message.match(/^API_ERROR_\d+:\s*(.+)$/);
  if (apiErrorMatch?.[1]) {
    return apiErrorMatch[1].trim();
  }

  const requestErrorMatch = message.match(/^Request failed:\s*\d+\s*-\s*(.+)$/);
  if (requestErrorMatch?.[1]) {
    return requestErrorMatch[1].trim();
  }

  return message.trim();
}

export function getUserFacingErrorMessage(
  error: unknown,
  fallbackMessage: string
): string {
  if (typeof error === "string") {
    const message = unwrapApiErrorMessage(error);
    return message || fallbackMessage;
  }

  if (error instanceof Error) {
    const message = unwrapApiErrorMessage(error.message);
    if (!message) {
      return fallbackMessage;
    }

    if (
      message.startsWith("AUTH_") ||
      message.startsWith("NETWORK_ERROR") ||
      message.startsWith("SERVER_ERROR")
    ) {
      return fallbackMessage;
    }

    return message;
  }

  return fallbackMessage;
}
