const VOICE_WS = "wss://" + document.location.host + "/ws/voice/" + window.localStorage.getItem('auth_token')
const PROFILE_WS = "wss://" + document.location.host + "/ws/" + window.localStorage.getItem('auth_token')
const GAME_WS = "wss://" + document.location.host + "/ws/" + window.localStorage.getItem('auth_token')
const WORLD_WS = "wss://" + document.location.host + "/ws/worldchat/" + window.localStorage.getItem('auth_token')