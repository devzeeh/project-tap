// This file defines the MessageData struct, which is used to pass error and success messages
// from the Go code to the HTML templates for the admin pages (like addCards.html and deactivateCard.html).
// It allows us to easily display feedback to the admin after they submit a form, such as whether the operation was successful or if there were any errors.
// The MessageData struct has two fields: Error and Success, both of which are strings.
package message

// This file defines the data structures used to pass information
// between the Go code and the HTML templates for the account-related pages.
type MessageData struct {
	Error   string
	Success string
}
