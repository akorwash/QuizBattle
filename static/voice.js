var audioContext;
var play = false;
$("#onYourMic").click(function() {
    var BUFF_SIZE = 16384;
    var errSpan = document.getElementById('worldchaterrorSumm')

              var audioInput = null,
                  microphone_stream = null,
                  gain_node = null,
                  script_processor_node = null,
                  script_processor_fft_node = null,
                  analyserNode = null;

            if(play){
                audioContext.close();
                clearInterval(playInterval)
                play = false
                $("#onYourMic").removeClass('btn-success')
                $("#onYourMic").addClass('btn-danger')
                return
            }
            
            if(!audioContext || !play){
                audioContext = new AudioContext();
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
                        
                        const analyser = audioContext.createAnalyser();
                        analyser.smoothingTimeConstant = 0;
                        analyser.fftSize = 2048;
                        var buffer_length = analyser.frequencyBinCount;

                        audioContext.createMediaStreamSource(stream).connect(analyser);
                        var array_freq_domain = new Uint8Array(buffer_length);
                        var array_time_domain = new Uint8Array(buffer_length);
                        
                       playInterval = setInterval(() => {                        
                            analyser.getByteFrequencyData(array_freq_domain);
                            analyser.getByteTimeDomainData(array_time_domain);

                            console.log(array_freq_domain)
                            console.log(array_time_domain)
                          }, 1000);
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
});


