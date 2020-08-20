var gamestreamconn;
var gamesLoaded = [];


function getLoadGamePath() {
    return '/game/mybattles'
 }
 
function emptyError(){
    var errSpan = document.getElementById('errorSumm')
    errSpan.innerText = ""
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
         });
     })
       .fail(function(failObj){
         var data = JSON.parse(failObj.responseText);
         var errSpan = document.getElementById('errorSumm')
         errSpan.innerText = data.error
       });
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
            if (doScroll) {
                games_list.scrollTop = games_list.scrollHeight - games_list.clientHeight;
            }

            if (!gamestreamconn) {
                return false;
            }
            
            gamestreamconn.send("newgame_"+element.ID);
            window.location.href = "/battle/"+element.ID
    })
    .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
    });
    $("#creategame").prop('disabled', false);
}


function LoadStream(){
    if (window["WebSocket"]) {
        gamestreamconn = new WebSocket("wss://" + document.location.host + "/ws/" + window.localStorage.getItem('auth_token'));
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


function processStreamCommand(command){
    if(command.includes("newgame")){
        LoadGame(command.split("_")[1])
    }

    if(command.includes("newjoin")){
        UpdateJoinGame(command.split("_")[1])
    }
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

function JoinGame(id){
    $.ajax({
        type: 'post',
        url: '/api/v1/game/join/'+id+"/any",
        headers: {"Authorization": "bearer "+ window.localStorage.getItem('auth_token')}
    })
    .done(function (element) {
        emptyError()
        gamestreamconn.send("newjoin_"+id);

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
};