package resources

//UserLogin user login model consumed by REST api
type UserLogin struct {
	Identifier string
	Password   string
}

//UserAccount login or create account response
type UserAccount struct {
	UserID       int64
	FullName     string
	Username     string
	MobileNumber string
	Email        string
	Token        string
}

//CreateAccountModel to do
type CreateAccountModel struct {
	Username     string
	FullName     string
	MobileNumber string
	Email        string
	Password     string
}

//UpdateAccountModel to do
type UpdateAccountModel struct {
	ID           int64
	Fullname     string
	YearOfBirth  int
	MonthOfBirth int
	DayOfBirth   int
}

//UserModel to do
type UserModel struct {
	ID       int64
	Fullname string
}
