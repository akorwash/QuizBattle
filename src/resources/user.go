package resources

// UserLogin user login model consumed by REST api
type UserLogin struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// UserAccount login or create account response
type UserAccount struct {
	UserID       int64  `json:"userId,string"`
	FullName     string `json:"fullName"`
	Username     string `json:"username"`
	MobileNumber string `json:"mobileNumber"`
	Email        string `json:"email"`
	YearOfBirth  int    `json:"yearOfBirth,omitempty"`
	MonthOfBirth int    `json:"monthOfBirth,omitempty"`
	DayOfBirth   int    `json:"dayOfBirth,omitempty"`
}

// CreateAccountModel to do
type CreateAccountModel struct {
	Username     string `json:"username"`
	FullName     string `json:"fullName"`
	MobileNumber string `json:"mobileNumber"`
	Email        string `json:"email"`
	Password     string `json:"password"`
}

// UpdateAccountModel to do
type UpdateAccountModel struct {
	FullName     string `json:"fullName"`
	YearOfBirth  int    `json:"yearOfBirth"`
	MonthOfBirth int    `json:"monthOfBirth"`
	DayOfBirth   int    `json:"dayOfBirth"`
}

// UserModel to do
type UserModel struct {
	ID          int64  `json:"id,string"`
	FullName    string `json:"fullName"`
	Team        int    `json:"team,omitempty"`
	IsBot       bool   `json:"isBot,omitempty"`
	BotStrategy string `json:"botStrategy,omitempty"`
}
