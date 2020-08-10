package engine

//IValidateInput to do
type IValidateInput interface {
	Validate(data string) bool
}
