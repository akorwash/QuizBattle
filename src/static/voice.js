
var voicechatStreamConn;


var audioContext;
var alltoHear = false;
var play = false;
var buf;        // Audio buffer
var BUFF_SIZE = 16384;
var audioInput = null,
microphone_stream = null,
gain_node = null,
script_processor_node = null,
script_processor_fft_node = null,
analyserNode = null;
$("#onYourMic").click(function() {
    var errSpan = document.getElementById('worldchaterrorSumm')
    

            if(play){
                audioContext.close();
               // clearInterval(playInterval)
                play = false
                $("#onYourMic").removeClass('btn-success')
                $("#onYourMic").addClass('btn-danger')
                return
            }
            
            if(!audioContext || !play){
              audioContext= typeof AudioContext !== 'undefined' ? new AudioContext() : new webkitAudioContext();
            }
                
            audioContext.resume().then(() => {
              console.log('Playback resumed successfully');

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
                      // var mediaRecorder = new MediaRecorder(stream);
                     
                      // mediaRecorder.ondataavailable = function(e) {
                      //   if(e.data.size > 0){
                      //     var chunks = [];
                      //     chunks.push(e.data);
                      //     var id = {uid: window.localStorage.getItem('auth_uid')}
                      //     var blob = new Blob(chunks, { 'type' : 'audio/ogg; codecs=opus' });
                      //     voicechatStreamConn.send(blob);
                      //   }
                      // };

                     

                      // playInterval = setInterval(function() {
                      //   if(mediaRecorder.state != "recording"){
                      //     mediaRecorder.start();
                      //   }
                      // })


                      // // Stop recording after 1 second and broadcast it to server
                      //  setInterval(function() {
                      //   mediaRecorder.stop()
                      // }, 1000);

                        const analyser = audioContext.createAnalyser();
                        analyser.smoothingTimeConstant = 0;
                        analyser.fftSize = BUFF_SIZE;

                        var source = audioContext.createMediaStreamSource(stream)
                        const processor = audioContext.createScriptProcessor(analyser.frequencyBinCount, 1, 1);
                        source.connect(processor);
                        processor.connect(audioContext.destination);

                        processor.onaudioprocess = function(e) {                          
                          voicechatStreamConn.send(bufferToWave(e.inputBuffer,e.inputBuffer.sampleRate));
                        };
                    },
                    function(e) {
                        play = false
                      errSpan.innerText ="Error capturing audio."
                    }
                  );

              } else {
                play = false
                errSpan.innerText ="getUserMedia not supported in this browser."}
              });



            
});


$("#hearTheWorld").click(function() {
  if(alltoHear){
    alltoHear = false;
  }else{
    alltoHear = true;
  }
});

// Convert an AudioBuffer to a Blob using WAVE representation
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

function show_some_data(given_typed_array, num_row_to_display, label) {

  var size_buffer = given_typed_array.length;
  var index = 0;
  var max_index = num_row_to_display;

  console.log("__________ " + label);

  for (; index < max_index && index < size_buffer; index += 1) {

      console.log(given_typed_array[index]);
  }
}

function process_microphone_buffer(event) { // invoked by event loop

  var i, N, inp, microphone_output_buffer;

  microphone_output_buffer = event.inputBuffer.getChannelData(0); // just mono - 1 channel for now

  // microphone_output_buffer  <-- this buffer contains current gulp of data size BUFF_SIZE

  show_some_data(microphone_output_buffer, 5, "from getChannelData");
}

function start_microphone(stream){

  gain_node = audioContext.createGain();
  gain_node.connect( audioContext.destination );

  microphone_stream = audioContext.createMediaStreamSource(stream);
  microphone_stream.connect(gain_node); 

  script_processor_node = audioContext.createScriptProcessor(BUFF_SIZE, 1, 1);
  script_processor_node.onaudioprocess = process_microphone_buffer;

  microphone_stream.connect(script_processor_node);

  // --- enable volume control for output speakers

  document.getElementById('volume').addEventListener('change', function() {

      var curr_volume = this.value;
      gain_node.gain.value = curr_volume;

      console.log("curr_volume ", curr_volume);
  });

  // --- setup FFT

  script_processor_fft_node = audioContext.createScriptProcessor(2048, 1, 1);
  script_processor_fft_node.connect(gain_node);

  analyserNode = audioContext.createAnalyser();
  analyserNode.smoothingTimeConstant = 0;
  analyserNode.fftSize = 2048;

  microphone_stream.connect(analyserNode);

  analyserNode.connect(script_processor_fft_node);

  script_processor_fft_node.onaudioprocess = function() {

    // get the average for the first channel
    var array = new Uint8Array(analyserNode.frequencyBinCount);
    analyserNode.getByteFrequencyData(array);
    
    //we will use this
    var voiceData = new Uint8Array(analyserNode.frequencyBinCount);
    analyserNode.getByteTimeDomainData(voiceData);

    // draw the spectrogram
    if (microphone_stream.playbackState == microphone_stream.PLAYING_STATE) {
        show_some_data(array, 5, "from fft");
    }
  };
}


function processvoice(arrayBuffer){
  //message.UserId != window.localStorage.getItem('auth_uid')
  if(true){
    if(alltoHear){

      const reader = new FileReader();
      reader.addEventListener('loadend', () => {
        // reader.result contains the contents of blob as a typed array
        console.log(reader.result)

      });
      reader.readAsArrayBuffer(arrayBuffer);

    var blob = new Blob([arrayBuffer], { 'type' : 'audio/ogg; codecs=opus' });
    var audio = document.createElement('audio');
    audio.src = window.URL.createObjectURL(blob);
    audio.play();
    }
  }
}

function getBufferCallback( buffers, len, sample ) {
  var newSource = audioContext.createBufferSource();
  var newBuffer = audioContext.createBuffer( 1, len, sample );
  newBuffer.getChannelData(0).set(buffers);
  newSource.buffer = newBuffer;

  newSource.connect( audioContext.destination );
  newSource.start();
}

// Utility to add "compressed" to the uploaded file's name
function generateFileName() {
	var origin_name = fileInput.files[0].name;
	var pos = origin_name.lastIndexOf('.');
	var no_ext = origin_name.slice(0, pos);

	return no_ext + ".compressed.wav";
}

function ab2str(buffer) {
  var bufView = new Uint16Array(buffer);
  var length = bufView.length;
  var result = '';
  var addition = Math.pow(2,16)-1;

  for(var i = 0;i<length;i+=addition){

      if(i + addition > length){
          addition = length - i;
      }
      result += String.fromCharCode.apply(null, bufView.subarray(i,i+addition));
  }

  return result;
}
function str2ab(str) {
  var buf = new ArrayBuffer(str.length*2); // 2 bytes for each char
  var bufView = new Uint16Array(buf);
  for (var i=0, strLen=str.length; i < strLen; i++) {
    bufView[i] = str.charCodeAt(i);
  }
  return buf;
}

function LoadVoiceChatStream(){
  if (window["WebSocket"]) {
    voicechatStreamConn = new WebSocket("wss://" + document.location.host + "/ws/voice/" + window.localStorage.getItem('auth_token'));
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


LoadVoiceChatStream()