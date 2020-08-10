package handler

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

//IValidateInput use to validate inputes from users, such as email validator and username validator
type IValidateInput interface {
	Validate(data string) bool
}

var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
var mobileRegex = regexp.MustCompile(`^[0-9 ]+$`)

// IsEmailValid checks if the email provided passes the required structure and length.
func IsEmailValid(e string) bool {
	if len(e) < 3 && len(e) > 254 {
		return false
	}
	return emailRegex.MatchString(e)
}

//IsMobileNumberValid checks if the mobile number provided passes the required structure and length.
func IsMobileNumberValid(e string) bool {
	if len(e) != 11 {
		return false
	}
	return mobileRegex.MatchString(e)
}

//ValidateMobile validate the mobile number
type ValidateMobile struct {
}

//Validate implmention of IValidateInput for mobile number
func (validationModel ValidateMobile) Validate(data string) bool {
	return IsMobileNumberValid(data)
}

//ValidateEmail validate the email
type ValidateEmail struct {
}

//Validate implmention of IValidateInput for email
func (validationModel ValidateEmail) Validate(data string) bool {

	return IsEmailValid(data)
}

//ValidatePassword validate the password passed our criteria
type ValidatePassword struct {
}

//Validate implmention of IValidateInput for password
func (validationModel ValidatePassword) Validate(pass string) bool {
	var (
		upp, low, num, sym bool
		tot                uint8
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

	if !upp || !low || !num || !sym || tot < 8 {
		return false
	}

	return true
}

//ValidateUsername validate the username passed our criteria
type ValidateUsername struct {
}

//Validate implmention of IValidateInput for username
func (validationModel ValidateUsername) Validate(data string) bool {
	if len(data) < 5 {
		return false
	}

	if _, err := strconv.Atoi(string([]rune(data)[1])); err == nil {
		return false
	}
	return len(data) == len(strings.Replace(data, " ", "", -1))
}
