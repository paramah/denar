// Package notify groups the adapters that send notifications to channels
// outside the app: smtp, webhook, ntfy, telegram and matrix, each a
// usecase.Notifier in its own package. notifyhttp holds what the HTTP ones
// share.
package notify
