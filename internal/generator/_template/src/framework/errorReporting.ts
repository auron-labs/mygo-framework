import * as Sentry from "@sentry/browser";

// startErrorReporting reports uncaught errors and unhandled rejections to the
// DSN the Go side was started with. It does nothing, and sends nothing, when
// there is no DSN or it cannot be loaded.
export const startErrorReporting = async (loadDsn: () => Promise<string>): Promise<boolean> => {
  let dsn: string;
  try {
    dsn = await loadDsn();
  } catch {
    return false;
  }
  if (dsn === "") return false;

  Sentry.init({
    dsn,
    defaultIntegrations: false,
    integrations: [Sentry.globalHandlersIntegration(), Sentry.dedupeIntegration()],
  });
  return true;
};

// reportError sends an unexpected failure to error reporting, when a DSN was
// configured.
export const reportError = (error: unknown): void => {
  Sentry.captureException(error);
};

// flushErrorReports delivers pending events, for use as the page unloads.
export const flushErrorReports = (): Promise<boolean> => Sentry.flush(2000);
