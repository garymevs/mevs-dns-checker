// Package resolvers provides a curated set of public DNS resolvers
// used to approximate multi-location propagation checks.
package resolvers

// Resolver is a named public DNS endpoint.
type Resolver struct {
	Name    string
	Address string // host:port
	Region  string
}

// Default returns the built-in public resolver list.
func Default() []Resolver {
	return []Resolver{
		{Name: "Cloudflare", Address: "1.1.1.1:53", Region: "Global"},
		{Name: "Cloudflare (2)", Address: "1.0.0.1:53", Region: "Global"},
		{Name: "Google", Address: "8.8.8.8:53", Region: "Global"},
		{Name: "Google (2)", Address: "8.8.4.4:53", Region: "Global"},
		{Name: "Quad9", Address: "9.9.9.9:53", Region: "Global"},
		{Name: "Quad9 (2)", Address: "149.112.112.112:53", Region: "Global"},
		{Name: "OpenDNS", Address: "208.67.222.222:53", Region: "Global"},
		{Name: "OpenDNS (2)", Address: "208.67.220.220:53", Region: "Global"},
		{Name: "Control D", Address: "76.76.2.0:53", Region: "Global"},
		{Name: "AdGuard", Address: "94.140.14.14:53", Region: "EU"},
		{Name: "AdGuard (2)", Address: "94.140.15.15:53", Region: "EU"},
		{Name: "CleanBrowsing", Address: "185.228.168.9:53", Region: "Global"},
		{Name: "Verisign", Address: "64.6.64.6:53", Region: "US"},
		{Name: "Yandex", Address: "77.88.8.8:53", Region: "RU"},
		{Name: "Shecan", Address: "178.22.122.100:53", Region: "IR"},
		{Name: "DNS.SB", Address: "185.222.222.222:53", Region: "EU"},
	}
}
