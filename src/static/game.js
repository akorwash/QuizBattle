var gamestreamconn;
var gamesLoaded = [];

function getLoadGamePath() {
   if ($('#public_game').is(':checked')) {
       return '/game/publicbattles'
   }else{
       return '/game/mybattles'
   }     
}

function LoadGames(){
    $.ajax({
        type: 'get',
        url: getLoadGamePath(),
        headers: {"Authorization": "bearer "+ window.localStorage.getItem('auth_token')}
    })
    .done(function (data) {
        if(data)
        data.forEach(element => {
            var games_list = document.getElementById("games_list");
            var doScroll = games_list.scrollTop > games_list.scrollHeight - games_list.clientHeight - 1;
            
            var devElm = `<div class="media text-muted pt-3">
            <p class="media-body mb-0 small lh-125">
                <strong class="d-block text-gray-dark">Join `+element.User.Fullname+` Battle </strong>
            </p>
            `

            if(element.IsActive){
                devElm = devElm + `<a id="game_`+element.ID+`" onclick="JoinGame(`+element.ID+`)" class="joingame btn btn-sm btn-primary">Join `+element.JoinedUser.length+` Players</a>
                </div>`
            }else{
                devElm = devElm + `<a id="game_`+element.ID+`" onclick="JoinGame(`+element.ID+`)" class="joingame btn btn-sm btn-danger">Join `+element.JoinedUser.length+` Players [CLOSED GAME]</a>
                </div>`
            }
            $(games_list).append(devElm)
          gamesLoaded.push("" + element.ID)
            if (doScroll) {
                games_list.scrollTop = games_list.scrollHeight - games_list.clientHeight;
            }
        });
    })
      .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
      });
}

function LoadGame(id){
    emptyError()
    if(!gamesLoaded.includes(id)){
        $.ajax({
            type: 'get',
            url: getLoadGamePath(),
            headers: {"Authorization": "bearer "+ window.localStorage.getItem('auth_token')}
        })
        .done(function (data) {
            data.forEach(element => {
                if(element.ID == id){
                   
                    var games_list = document.getElementById("games_list");
                    var doScroll = games_list.scrollTop > games_list.scrollHeight - games_list.clientHeight - 1;
                    
                    $(games_list).append(`
    
                    <div class="media text-muted pt-3">
                        <p class="media-body pb-3 mb-0 small lh-125 border-bottom border-gray">
                            <strong class="d-block text-gray-dark">Join `+element.User.Fullname+` Battle </strong>
                        </p>
                        <a id="game_`+element.ID+`" onclick="JoinGame(`+element.ID+`)" class="joingame btn btn-sm btn-primary">Join `+element.JoinedUser.length+` Players</a>
                        </div>
                `)
    
                    if (doScroll) {
                        games_list.scrollTop = games_list.scrollHeight - games_list.clientHeight;
                    }
                }
            });
        })
          .fail(function(failObj){
            var data = JSON.parse(failObj.responseText);
            var errSpan = document.getElementById('errorSumm')
            errSpan.innerText = data.error
          });
    }
}

function CreateGame(){
    $("#creategame").prop('disabled', true);
    emptyError()
    var modgame = true;
    if($("#createdmodegame").prop("checked") == true){
        modgame = true;
    }
    else {
        modgame = false;
    }
    $.ajax({
        type: 'post',
        url: '/api/v1/game/new',
        contentType: 'application/json',
        data: '{"IsPublic":'+modgame+',"UserID": '+window.localStorage.getItem('auth_uid')+'}',
        headers: {"Authorization": "bearer "+ window.localStorage.getItem('auth_token')}
    })
    .done(function (element) {
            var games_list = document.getElementById("games_list");
            var doScroll = games_list.scrollTop > games_list.scrollHeight - games_list.clientHeight - 1;
            
            $(games_list).append(`
            <div class="media text-muted pt-3">
                <p class="media-body pb-3 mb-0 small lh-125 border-bottom border-gray">
                    <strong class="d-block text-gray-dark">Join `+element.User.Fullname+` Battle </strong>
                </p>
                <a id="game_`+element.ID+`" onclick="JoinGame(`+element.ID+`)" class="joingame btn btn-sm btn-primary">Join `+element.JoinedUser.length+` Players</a>
                </div>
          `)
          gamesLoaded.push("" + element.ID)
            if (doScroll) {
                games_list.scrollTop = games_list.scrollHeight - games_list.clientHeight;
            }

            if (!gamestreamconn) {
                return false;
            }

            var data = JSON.stringify({
                UserId : window.localStorage.getItem('auth_uid'),
                Type: "newgame",
                GameId: element.ID
              })
            gamestreamconn.send(data);
            window.location.href = "/battle/"+element.ID
    })
    .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
    });
    $("#creategame").prop('disabled', false);
}

function processStreamCommand(command){
    var prom = command.text()
    prom.then(function(mesaageStr){
        var mesaage = JSON.parse(mesaageStr);

        if(mesaage.Type === "newgame"){
            LoadGame(mesaage.GameId)
        }

        if(mesaage.Type === "newjoin"){
            UpdateJoinGame(mesaage.GameId)
        }
    });
}

function UpdateJoinGame(id){
    emptyError()
    $.ajax({
        type: 'get',
        url: getLoadGamePath(),
        headers: {"Authorization": "bearer "+ window.localStorage.getItem('auth_token')}
    })
    .done(function (data) {
        data.forEach(element => {
            if(element.ID == id){
               
                var joinlink = document.getElementById("game_"+id)
                joinlink.innerText = `Join `+ element.JoinedUser.length +` Players`
            }
        });
    })
      .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
      });
}

function LoadStream(){
    if (window["WebSocket"]) {
        gamestreamconn = new WebSocket(GAME_WS);
        gamestreamconn.onclose = function (evt) {
            var errSpan = document.getElementById('errorSumm')
            errSpan.innerText = "Connection close with server"
        };
        gamestreamconn.onmessage = function (evt) {
            processStreamCommand(evt.data)
        };
    }else{
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = "your browser dosn't support websokets so you have to refresh your page every time"
    }
}

function emptyError(){
    var errSpan = document.getElementById('errorSumm')
    errSpan.innerText = ""
}

function JoinGame(id){
    $.ajax({
        type: 'post',
        url: '/api/v1/game/join/'+id+"/any",
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

        //redirct player to game page
        window.location.href = "/battle/"+id
    })
    .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
      });
}

window.onload = function () {
    emptyError()
    LoadGames()
    LoadStream()

    $("#creategame").click(function() {
        CreateGame()
      });

    $('input[type=radio][name=gameownder]').change(function() {
        $('#games_list').empty()
        emptyError()
        LoadGames()
    });

     
};