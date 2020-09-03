
var gameinformation = null;
var gamestreamconn;

function loadgameinfo() {
    
    var battlepage = document.getElementById('battlepage')
    $(battlepage).addClass('loader')
    $(battlepage).css('display','block')

    var gameId = window.location.href.split('/')[4]
    $.ajax({
        type: 'get',
        url: '/game/publicbattles',
        headers: {"Authorization": "bearer "+ window.localStorage.getItem('auth_token')}
    })
    .done(function (data) {
        data.forEach(element => {
            if(element.ID == gameId){
                gameinformation = element
            }
        });
    })
      .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
        $(battlepage).removeClass('loader')
        $(battlepage).css('display','none')
      });
}

function titleedit() {
    if(gameinformation){
        var pageTitle = document.getElementById('pagetile')
        pageTitle.innerText = "Quiz Battle - "+ gameinformation.User.Fullname +" Battle"

        var battleTitle = document.getElementById('battleTitle')
        var startGameButton = document.getElementById('startgameButton')
        battleTitle.innerText =  gameinformation.User.Fullname +" Battle"


        gameinformation.JoinedUser.forEach(function(user){
            var joined_users_list = document.getElementById("battle_Joined_Users_container");
            var user_fiv = `<div class="bold">
            <a href="" style="color:black; text-decoration:none;" ><span>`+user.Fullname+`</span></a><span style="color:green" class="p-3">Online</span>
          </div>`

            $(joined_users_list).append(user_fiv)
        })
        $(battlepage).removeClass('loader')
        $(battlepage).css('display','none')
        
        if(gameinformation.User.ID != window.localStorage.getItem('auth_uid')){
            $(startGameButton).remove()
        }else{
            $(startGameButton).css('display','block')
        }

        $('#battleviewsection > button.btn.btn-sm.btn-danger').css('display','block')
    }
}

window.onload = function () {
    loadgameinfo()
    setTimeout(titleedit,1500); // run donothing after 1.5 seconds

    if (window["WebSocket"]) {
        gamestreamconn = new WebSocket(GAME_WS);
        gamestreamconn.onclose = function (evt) {
            var errSpan = document.getElementById('errorSumm')
            errSpan.innerText = "Connection close with server"
        };


    }else{
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = "your browser dosn't support websokets so you have to refresh your page every time"
    }
};

function emptyError(){
    var errSpan = document.getElementById('errorSumm')
    errSpan.innerText = ""
}

function exitBattle(){
    var id = window.location.href.split('/')[4]
    $.ajax({
        type: 'post',
        url: '/api/v1/game/exit/'+id+"",
        headers: {"Authorization": "bearer "+ window.localStorage.getItem('auth_token')}
    })
    .done(function (element) {
        emptyError()
        var data = JSON.stringify({
            UserId : window.localStorage.getItem('auth_uid'),
            Type: "newjoin",
            GameId: id
          })
        gamestreamconn.send(data);
        //redirct player to home page
        window.location.href = "/"
    })
    .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
      });
}

function startBattle(){

}