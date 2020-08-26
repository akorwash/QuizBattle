
// Utility to convert audio buffer to string
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

// Utility to convert string to audio buffer
function str2ab(str) {
    var buf = new ArrayBuffer(str.length*2); // 2 bytes for each char
    var bufView = new Uint16Array(buf);
    for (var i=0, strLen=str.length; i < strLen; i++) {
      bufView[i] = str.charCodeAt(i);
    }
    return buf;
}

// Utility to add "compressed" to the uploaded file's name
function generateFileName() {
	var origin_name = fileInput.files[0].name;
	var pos = origin_name.lastIndexOf('.');
	var no_ext = origin_name.slice(0, pos);

	return no_ext + ".compressed.wav";
}
