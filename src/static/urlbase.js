var host = window.location.hostname;
var pref_ws = "wss://"

if(host == 'localhost') {
    pref_ws = "ws://"
}

const VOICE_WS = pref_ws + document.location.host + "/ws/voice/" + window.localStorage.getItem('auth_token')
const PROFILE_WS = pref_ws + document.location.host + "/ws/" + window.localStorage.getItem('auth_token')
const GAME_WS = pref_ws + document.location.host + "/ws/" + window.localStorage.getItem('auth_token')
const WORLD_WS = pref_ws + document.location.host + "/ws/worldchat/" + window.localStorage.getItem('auth_token')