package controller

import "net/http"

//HomeController home page controller
type HomeController struct{}

//HomePage home page http requst handler
func (controller *HomeController) HomePage(w http.ResponseWriter, r *http.Request) {

	strHTML := `<!DOCTYPE html>
	<html>
	<style>
	body, html {
	  height: 100%;
	  margin: 0;
	}
	
	.bgimg {
	  background-image: url('https://www.w3schools.com/w3images/forestbridge.jpg');
	  height: 100%;
	  background-position: center;
	  background-size: cover;
	  position: relative;
	  color: white;
	  font-family: "Courier New", Courier, monospace;
	  font-size: 25px;
	}
	
	.topleft {
	  position: absolute;
	  top: 0;
	  left: 16px;
	}
	
	.bottomleft {
	  position: absolute;
	  bottom: 0;
	  left: 16px;
	}
	
	.middle {
	  position: absolute;
	  top: 50%;
	  left: 50%;
	  transform: translate(-50%, -50%);
	  text-align: center;
	}
	
	hr {
	  margin: auto;
	  width: 40%;
	}
	</style>
	<body>
	
	<div class="bgimg">
	  <div class="topleft">
		<p>Quiz Battle</p>
	  </div>
	  <div class="middle">
		<h1>COMING SOON</h1>
		<hr>
		<p>Korwash Here Play With GO Lang</p>
	  </div>
	  <div class="bottomleft">
		<p>Quiz Battle</p>
	  </div>
	</div>
	
	</body>
	</html>`
	b := []byte(strHTML)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}
