package handler

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// IValidateInput use to validate inputes from users, such as email validator and username validator
type IValidateInput interface {
	Validate(data string) bool
}

var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$")
var mobileRegex = regexp.MustCompile(`^\+?[0-9]{8,15}$`)
var usernameRegex = regexp.MustCompile(`^[\p{L}][\p{L}\p{N}_.-]{4,31}$`)

// IsEmailValid checks if the email provided passes the required structure and length.
func IsEmailValid(e string) bool {
	e = strings.TrimSpace(e)
	if len(e) < 3 || len(e) > 254 {
		return false
	}
	return emailRegex.MatchString(e)
}

// IsMobileNumberValid checks if the mobile number provided passes the required structure and length.
func IsMobileNumberValid(e string) bool {
	return mobileRegex.MatchString(strings.TrimSpace(e))
}

// ValidateMobile validate the mobile number
type ValidateMobile struct {
}

// Validate implmention of IValidateInput for mobile number
func (validationModel ValidateMobile) Validate(data string) bool {
	return IsMobileNumberValid(data)
}

// ValidateEmail validate the email
type ValidateEmail struct {
}

// Validate implmention of IValidateInput for email
func (validationModel ValidateEmail) Validate(data string) bool {

	return IsEmailValid(data)
}

// ValidatePassword validate the password passed our criteria
type ValidatePassword struct {
}

// Validate implmention of IValidateInput for password
func (validationModel ValidatePassword) Validate(pass string) bool {
	if len(pass) < 10 || len(pass) > 72 || !utf8.ValidString(pass) {
		return false
	}

	var (
		upp, low, num, sym bool
		tot                int
	)

	for _, char := range pass {
		switch {
		case unicode.IsUpper(char):
			upp = true
			tot++
		case unicode.IsLower(char):
			low = true
			tot++
		case unicode.IsNumber(char):
			num = true
			tot++
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			sym = true
			tot++
		default:
			return false
		}
	}

	if !upp || !low || !num || !sym || tot < 10 {
		return false
	}

	return true
}

// ValidateUsername validate the username passed our criteria
type ValidateUsername struct {
}

// Validate implmention of IValidateInput for username
func (validationModel ValidateUsername) Validate(data string) bool {
	return usernameRegex.MatchString(strings.TrimSpace(data))
}

// IsFullNameValid accepts human names while rejecting control characters and
// unreasonable payload sizes.
func IsFullNameValid(name string) bool {
	name = strings.TrimSpace(name)
	count := utf8.RuneCountInString(name)
	if count < 2 || count > 80 || !utf8.ValidString(name) {
		return false
	}
	for _, char := range name {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}
