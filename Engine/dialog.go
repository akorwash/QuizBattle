package engine

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

//IValidateInput to do
type IValidateInput interface {
	Validate(data string) bool
}

//MainDialog to do
func MainDialog() {
	fmt.Println("Please Choice from this Options: ")
	fmt.Println("1: Create Account")
	fmt.Println("2: Login")
	fmt.Println("3: Clear Console and Relaod")
	fmt.Println("4: Exit the game")

}

//ExitTheGame to do
func ExitTheGame() {
	fmt.Println("Thanks for your time.....")
	for i := 0; i < 5; i++ {
		time.Sleep(250 * time.Millisecond)
		fmt.Print("-")
	}
	fmt.Print(" Exit ")
	for i := 0; i < 5; i++ {
		time.Sleep(250 * time.Millisecond)
		fmt.Print("-")
	}
}

//StartTheGame to do
func StartTheGame() {
	fmt.Print("Loading ")
	for i := 0; i < 10; i++ {
		time.Sleep(250 * time.Millisecond)
		fmt.Print("-")
	}
	fmt.Println("-")
}

//StartSessionForUser to do
func StartSessionForUser() {
	fmt.Print("Loading ")
	for i := 0; i < 10; i++ {
		time.Sleep(250 * time.Millisecond)
		fmt.Print("-")
	}
	fmt.Println("-")
}

//ReadConsoleMessage to do
func ReadConsoleMessage() {
	fmt.Print("\n:>  ")
}

//ReadString to do
func ReadString() string {
	var _str string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		_str = scanner.Text()
		if len(_str) > 0 {
			break
		}
	}
	return _str
}

//ReadStringWithValidation to do
func ReadStringWithValidation(validationModel IValidateInput) string {
	var _str string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		_str = scanner.Text()
		if len(_str) > 0 {
			if validationModel.Validate(_str) {
				break
			} else {
				fmt.Println("Please Insure you enterd valid data \n ")
				fmt.Println("Enter Your Input Again")
				ReadConsoleMessage()
			}
		}
	}
	return _str
}
