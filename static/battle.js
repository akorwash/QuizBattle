
var gameinformation = null;

function loadgameinfo() {
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
      });
}

function titleedit() {
    if(gameinformation){
        var pageTitle = document.getElementById('pagetile')
        pageTitle.innerText = "Quiz Battle - "+ gameinformation.User.Fullname +" Battle"

        var battleTitle = document.getElementById('battleTitle')
        battleTitle.innerText =  gameinformation.User.Fullname +" Battle"
    }
}

window.onload = function () {
    loadgameinfo()
    setTimeout(titleedit,1500); // run donothing after 1.5 seconds
};
