package engine

import (
	"bufio"
	"fmt"
	"os"
)

//MainDialog to do
func MainDialog() {
	fmt.Println("Please Choice from this Options: ")
	fmt.Println("1: Create Account")
	fmt.Println("2: Login")
	fmt.Println("3: Clear Console and Relaod")

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
