if(window.localStorage.getItem('auth_token')){
    document.location.href = '/'
  }

  $(function () {

    $('#loginform').on('submit', function (e) {
      var errSpan = document.getElementById('errorSumm')
      errSpan.innerText = ""

      e.preventDefault();

      $.ajax({
        type: 'post',
        url: '/user/login',
        data: $('form').serialize()
      })
      .done(function (data) {
        window.localStorage.setItem('auth_token', data.Token);
        window.localStorage.setItem('auth_fullname', data.FullName);
        window.localStorage.setItem('auth_username', data.Username);
        window.localStorage.setItem('auth_mobile', data.MobileNumber);
        window.localStorage.setItem('auth_email', data.Email);
        window.localStorage.setItem('auth_uid', data.UserID);
        document.location.href = '/'
        })
      .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
      });

    });

    $('#signupform').on('submit', function (e) {
      var errSpan = document.getElementById('errorSumm')
      errSpan.innerText = ""

      e.preventDefault();

      $.ajax({
        type: 'post',
        url: '/user/createuser',
        data: $('form').serialize()
      })
      .done(function (data) {
        window.localStorage.setItem('auth_token', data.Token);
        window.localStorage.setItem('auth_fullname', data.FullName);
        window.localStorage.setItem('auth_username', data.Username);
        window.localStorage.setItem('auth_mobile', data.MobileNumber);
        window.localStorage.setItem('auth_email', data.Email);
        window.localStorage.setItem('auth_uid', data.UserID);
        document.location.href = '/'
        })
      .fail(function(failObj){
        var data = JSON.parse(failObj.responseText);
        var errSpan = document.getElementById('errorSumm')
        errSpan.innerText = data.error
      });

    });
  });