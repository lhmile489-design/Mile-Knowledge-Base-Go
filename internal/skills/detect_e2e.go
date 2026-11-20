//go:build e2e

package skills

// e2e builds read the fake environment so acceptance scripts never touch the
// real tool directories or the real registry (技术设计 15.7).
const fakeInputsEnabled = true
