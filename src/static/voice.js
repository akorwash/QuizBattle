var voicechatStreamConn;
var audioContext;
var alltoHear = false;
var play = false;
var buf;
var BUFF_SIZE = 16384;
var audioInput = null;
var microphone_stream = null;

$("#onYourMic").click(function() {
  var errSpan = document.getElementById('worldchaterrorSumm')
    

  if(play){
      audioContext.close();
      play = false
      $("#onYourMic").removeClass('btn-success')
      $("#onYourMic").addClass('btn-danger')
      return
  }
  
  if(!audioContext || !play){
    audioContext= typeof AudioContext !== 'undefined' ? new AudioContext() : new webkitAudioContext();
  }
  
  audioContext.resume().then(() => {
    $("#onYourMic").addClass('btn-success')
    $("#onYourMic").removeClass('btn-danger')
    play = true
    
  if (!navigator.getUserMedia){
    navigator.getUserMedia = ( navigator.getUserMedia    || navigator.webkitGetUserMedia ||
      navigator.mozGetUserMedia ||navigator.msGetUserMedia);    
  }
    if (navigator.getUserMedia){
        navigator.getUserMedia({audio:true}, 
          function(stream) {
              const analyser = audioContext.createAnalyser();
              analyser.smoothingTimeConstant = 0;
              analyser.fftSize = BUFF_SIZE;

              var source = audioContext.createMediaStreamSource(stream)
              const processor = audioContext.createScriptProcessor(analyser.frequencyBinCount, 1, 1);
              source.connect(processor);
              processor.connect(audioContext.destination);
              processor.onaudioprocess = function(e) {     
                if (voicechatStreamConn.readyState === WebSocket.OPEN) {
                  voicechatStreamConn.send(bufferToWave(e.inputBuffer,e.inputBuffer.sampleRate));
                }                     
              };
          },
          function(e) {
              play = false
            errSpan.innerText ="Error capturing audio."
          }
        )
    } else {
      play = false
      errSpan.innerText ="getUserMedia not supported in this browser."
    }
  });
});

$("#hearTheWorld").click(function() {
  if(alltoHear){
    alltoHear = false;
  }else{
    alltoHear = true;
  }
});

//Convert an AudioBuffer to a Blob using WAVE representation
function bufferToWave(abuffer, len) {
  var numOfChan = abuffer.numberOfChannels,
      length = len * numOfChan * 2 + 44,
      buffer = new ArrayBuffer(length),
      view = new DataView(buffer),
      channels = [], i, sample,
      offset = 0,
      pos = 0;

  // write WAVE header
  setUint32(0x46464952);                         // "RIFF"
  setUint32(length - 8);                         // file length - 8
  setUint32(0x45564157);                         // "WAVE"

  setUint32(0x20746d66);                         // "fmt " chunk
  setUint32(16);                                 // length = 16
  setUint16(1);                                  // PCM (uncompressed)
  setUint16(numOfChan);
  setUint32(abuffer.sampleRate);
  setUint32(abuffer.sampleRate * 2 * numOfChan); // avg. bytes/sec
  setUint16(numOfChan * 2);                      // block-align
  setUint16(16);                                 // 16-bit (hardcoded in this demo)

  setUint32(0x61746164);                         // "data" - chunk
  setUint32(length - pos - 4);                   // chunk length

  // write interleaved data
  for(i = 0; i < abuffer.numberOfChannels; i++)
    channels.push(abuffer.getChannelData(i));

  while(pos < length) {
    for(i = 0; i < numOfChan; i++) {             // interleave channels
      sample = Math.max(-1, Math.min(1, channels[i][offset])); // clamp
      sample = (0.5 + sample < 0 ? sample * 32768 : sample * 32767)|0; // scale to 16-bit signed int
      view.setInt16(pos, sample, true);          // write 16-bit sample
      pos += 2;
    }
    offset++                                     // next source sample
  }


  return  new Blob([buffer], { 'type' : 'audio/ogg; codecs=opus' });

  function setUint16(data) {
    view.setUint16(pos, data, true);
    pos += 2;
  }

  function setUint32(data) {
    view.setUint32(pos, data, true);
    pos += 4;
  }
}

//process ws message voice
function processvoice(arrayBuffer){
  //message.UserId != window.localStorage.getItem('auth_uid')
    if(alltoHear){
      var blob = new Blob([arrayBuffer], { 'type' : 'audio/ogg; codecs=opus' });
      var audio = document.createElement('audio');
      audio.src = window.URL.createObjectURL(blob);
      audio.volume = document.getElementById('volume').value
      audio.play();
    }
}

//start ws of voice
function LoadVoiceChatStream(){
  if (window["WebSocket"]) {
    voicechatStreamConn = new WebSocket(VOICE_WS);
    voicechatStreamConn.onclose = function (evt) {
          var errSpan = document.getElementById('worldchaterrorSumm')
          errSpan.innerText = "Connection close with server"
      };
    
      voicechatStreamConn.onmessage = function (evt) {
          processvoice(evt.data)
      };
  }else{
      var errSpan = document.getElementById('worldchaterrorSumm')
      errSpan.innerText = "your browser dosn't support websokets so you have to refresh your page every time"
  }
}

//insure that ws voice chat realtime openned
setInterval(function() {
  if(!voicechatStreamConn){
    LoadVoiceChatStream()
  }else{
    if (voicechatStreamConn.readyState === WebSocket.CLOSED) {
      LoadVoiceChatStream()
      emptyError()
    }
  }
});