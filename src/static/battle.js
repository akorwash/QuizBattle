
var gameinformation = null;

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
    }
}

window.onload = function () {
    loadgameinfo()
    setTimeout(titleedit,1500); // run donothing after 1.5 seconds
};


function exitBattle(){

}