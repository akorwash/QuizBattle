package resources

//UserLogin user login model consumed by REST api
type UserLogin struct {
	Identifier string
	Password   string
}

//UserAccount login or create account response
type UserAccount struct {
	Username     string
	MobileNumber string
	Email        string
	Token        string
}
