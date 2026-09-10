// Package phoneutil provides a single, shared phone-number normalization
// function.
//
// The original Node.js code normalized phone numbers in two different
// places with two different rules:
//   - member.controller.ts (login handler): "0812..." -> "62812...",
//     "+62812..." -> "62812..."
//   - member.repository.ts (findMemberByPhoneOrSpouse): only stripped a
//     leading "+"
//
// Because these could drift out of sync, a member logging in with
// "0812..." and a WhatsApp message arriving as "62812..." could
// technically be treated inconsistently by different code paths. This
// package centralizes the rule so every caller normalizes the same way.
package phoneutil

import "strings"

// Normalize converts a phone number to the canonical form used for
// lookups: digits only, Indonesian country code "62" prefix, no leading
// "+" or "0".
func Normalize(phone string) string {
	p := strings.TrimSpace(phone)
	p = strings.TrimPrefix(p, "+")

	switch {
	case strings.HasPrefix(p, "0"):
		p = "62" + p[1:]
	case strings.HasPrefix(p, "62"):
		// already canonical
	}

	return p
}
