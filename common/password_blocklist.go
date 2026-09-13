package common

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"slices"
	"strings"
)

// SHA-256 digests of the first 3,000 policy-matching entries for both the
// workspace (8+) and platform (15+) password policies. Plaintext passwords
// are never stored here. Source: danielmiessler/SecLists,
// Passwords/Common-Credentials/xato-net-10-million-passwords-1000000.txt.
// The digest set is used only for newly chosen passwords, never login checks.
//
//go:embed password_blocklist.sha256
var passwordBlocklistData string

var passwordBlocklist = strings.Fields(passwordBlocklistData)

func isCommonPassword(password string) bool {
	hash := sha256.Sum256([]byte(password))
	_, found := slices.BinarySearch(passwordBlocklist, hex.EncodeToString(hash[:]))
	return found
}

/*
MIT License

Copyright (c) 2018 Daniel Miessler

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
