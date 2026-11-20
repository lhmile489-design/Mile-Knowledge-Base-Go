//go:build !e2e

package skills

// fakeGithubBase is empty in production builds: the client always talks to the
// three GitHub hosts (PRD 15.4).
func fakeGithubBase() string { return "" }
