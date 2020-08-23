var worldcharStreamConn;
function LoadChatStream(){
    if (window["WebSocket"]) {
        worldcharStreamConn = new WebSocket("wss://" + document.location.host + "/ws/worldchat/" + window.localStorage.getItem('auth_token'));
        worldcharStreamConn.onclose = function (evt) {
            var errSpan = document.getElementById('worldchaterrorSumm')
            errSpan.innerText = "Connection close with server"
        };
      
        worldcharStreamConn.onmessage = function (evt) {
            processChatStreamCommand(evt.data)
        };
    }else{
        var errSpan = document.getElementById('worldchaterrorSumm')
        errSpan.innerText = "your browser dosn't support websokets so you have to refresh your page every time"
    }
}

function processChatStreamCommand(command){
    emptyError()
    var prom = command.text()
    prom.then(function(mesaageStr) {
        var mesaage = JSON.parse(mesaageStr);
        console.log(mesaage)

        var messages_list = document.getElementById("messages");
        var doScroll = messages_list.scrollTop > messages_list.scrollHeight - messages_list.clientHeight - 1;
        
        if(mesaage.Fullname === window.localStorage.getItem('auth_fullname')){

            var divSlic = `
            <div class="chat_list">
            <div class="chat_people">
              <div class="chat_img_left"> <img src="https://ptetutorials.com/images/user-profile.png" alt="sunil"> </div>
              <div class="chat_ib">
                <h5>`+mesaage.Fullname+` <span class="chat_date"></span></h5>`
                
                for (let index = 0; index < mesaage.Message.split('\n').length; index++) {
                    const messText = mesaage.Message.split('\n')[index];
                    divSlic = divSlic+ `<p class="text-left">`+messText+`</p>                    `
                }

                divSlic = divSlic +  `
                </div>
              </div>
            </div>
              `
            $(messages_list).append(divSlic)
        }else{
            var divSlic = `
            <div class="chat_list">
            <div class="chat_people">
              <div class="chat_img_right"> <img src="https://ptetutorials.com/images/user-profile.png" alt="sunil"> </div>
              <div class="chat_ib">
                <h5>`+mesaage.Fullname+` <span class="chat_date"></span></h5>`
                
                for (let index = 0; index < mesaage.Message.split('\n').length; index++) {
                    const messText = mesaage.Message.split('\n')[index];
                    divSlic = divSlic+ `<p class="text-right">`+messText+`</p>                    `
                }

                divSlic = divSlic +  `
                </div>
              </div>
            </div>
              `
            $(messages_list).append(divSlic)
        }
       
        if (doScroll) {
            messages_list.scrollTop = messages_list.scrollHeight - messages_list.clientHeight;
        }
      });
}

function emptyError(){
    var errSpan = document.getElementById('worldchaterrorSumm')
    errSpan.innerText = ""
}

function sendmessage(message){
    var data = JSON.stringify({
        Message: message,
        Fullname: window.localStorage.getItem('auth_fullname')
      })
      console.log(data)
      worldcharStreamConn.send(data);
}

emptyError()
LoadChatStream()

function fireClick(){
    var mess = $('#inputmessage').val()
    if(mess === ''){
        return
    }


    sendmessage(mess)
    $('#inputmessage').val('')
}


$("#sendmessage").click(function() {
    fireClick()
});


$('textarea').keyup(function (event) {
    if (event.keyCode == 13 && event.shiftKey) {
        var content = this.value;
        var caret = getCaret(this);
        this.value = content.substring(0,caret)+"\n"+content.substring(carent,content.length-1);
        event.stopPropagation();
        
   }else if(event.keyCode == 13)
   {
        fireClick()
   }
});


function getCaret(el) { 
if (el.selectionStart) { 
 return el.selectionStart; 
} else if (document.selection) { 
 el.focus(); 

 var r = document.selection.createRange(); 
 if (r == null) { 
   return 0; 
 } 

 var re = el.createTextRange(), 
     rc = re.duplicate(); 
 re.moveToBookmark(r.getBookmark()); 
 rc.setEndPoint('EndToStart', re); 

 return rc.text.length; 
}  
return 0; 
}