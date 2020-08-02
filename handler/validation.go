package handler

import "regexp"

var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
var mobileRegex = regexp.MustCompile(`^[0-9 ]+$`)

// IsEmailValid checks if the email provided passes the required structure and length.
func IsEmailValid(e string) bool {
	if len(e) < 3 && len(e) > 254 {
		return false
	}
	return emailRegex.MatchString(e)
}

//IsMobileNumberValid checks if the email provided passes the required structure and length.
func IsMobileNumberValid(e string) bool {
	if len(e) != 11 {
		return false
	}
	return mobileRegex.MatchString(e)
}

//ValidateMobile to do
type ValidateMobile struct {
}

//Validate to do
func (validationModel ValidateMobile) Validate(data string) bool {
	return IsMobileNumberValid(data)
}

//ValidateEmail to do
type ValidateEmail struct {
}

//Validate to do
func (validationModel ValidateEmail) Validate(data string) bool {

	return IsEmailValid(data)
}
