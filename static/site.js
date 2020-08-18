if(window.localStorage.getItem('auth_token')){
    document.getElementById('signInNav').style.display = 'none'
    document.getElementById('useracc').style.display = 'block'
    document.getElementById('useracc').style.color = '#00f3ff'
    document.getElementById('useracc').href = '/user/profile/' + window.localStorage.getItem('auth_fullname')
    document.getElementById('useracc').innerText = 'Welcome ' + window.localStorage.getItem('auth_fullname')
    document.getElementById('logout').style.display = 'block'
}

function Logout(){
    window.localStorage.removeItem('auth_token');
    window.localStorage.removeItem('auth_fullname');
    window.localStorage.removeItem('auth_username');
    window.localStorage.removeItem('auth_mobile');
    window.localStorage.removeItem('auth_email');
    window.localStorage.removeItem('auth_uid');

    window.location.reload()
}