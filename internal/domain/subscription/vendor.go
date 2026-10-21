// Package subscription models recurring costs: a subscription with its
// vendor, price history and billing cycle, the charges expected from it
// and how well a transaction matches one.
package subscription

// Vendor is a known provider of subscriptions. Future connectors find a
// subscription by its vendor and external reference.
type Vendor struct {
	Key  string
	Name string
}

// VendorOther is the vendor of everything not on the list.
const VendorOther = "other"

// Vendors is the closed list of vendors, in a stable order.
var Vendors = []Vendor{
	{"openai", "OpenAI"},
	{"anthropic", "Anthropic"},
	{"apple", "Apple"},
	{"google", "Google"},
	{"netflix", "Netflix"},
	{"spotify", "Spotify"},
	{"youtube", "YouTube"},
	{"disney", "Disney+"},
	{"hbo", "HBO Max"},
	{"amazon", "Amazon"},
	{"microsoft", "Microsoft"},
	{"adobe", "Adobe"},
	{"github", "GitHub"},
	{"icloud", "iCloud"},
	{"dropbox", "Dropbox"},
	{VendorOther, "Inne"},
}

// LookupVendor returns the vendor with the key.
func LookupVendor(key string) (Vendor, bool) {
	for _, v := range Vendors {
		if v.Key == key {
			return v, true
		}
	}
	return Vendor{}, false
}

// VendorName is the display name of the vendor key, or the key itself.
func VendorName(key string) string {
	if v, ok := LookupVendor(key); ok {
		return v.Name
	}
	return key
}
