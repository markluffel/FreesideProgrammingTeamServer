function fileSelected() {
	var files = document.getElementById('fileToUpload').files;
	document.getElementById('fileNotes').innerHTML = "";
	for (var i=0;i<files.length;i++)
	{ 
		var fileSize = 0;
		if (files[i].size > 1024 * 1024)
			fileSize = (Math.round(files[i].size * 100 / (1024 * 1024)) / 100).toString() + 'MB';
		else
			fileSize = (Math.round(files[i].size * 100 / 1024) / 100).toString() + 'KB';
		document.getElementById('fileNotes').innerHTML += 'Name: ' + files[i].name + "<br>";
		document.getElementById('fileNotes').innerHTML += 'Size: ' + fileSize + "<br>";
		document.getElementById('fileNotes').innerHTML += 'Type: ' + files[i].type + "<br>";
	}
}

      function uploadContest() {
        var fd = new( FormData);
	fd.append("fileToUpload", document.getElementById('fileToUpload').files);
	fd.append("contest", document.getElementById('contest').value);
	fd.append("secret", document.getElementById('Secret').value);
        var xhr = new XMLHttpRequest();
        xhr.upload.addEventListener("progress", uploadProgress, false);
        xhr.addEventListener("load", uploadComplete, false);
        xhr.addEventListener("error", uploadFailed, false);
        xhr.addEventListener("abort", uploadCanceled, false);
        xhr.open("POST", "judge");
        xhr.send(fd);
      }

      function uploadProgress(evt) {
        if (evt.lengthComputable) {
          var percentComplete = Math.round(evt.loaded * 100 / evt.total);
          document.getElementById('progressNumber').innerHTML = percentComplete.toString() + '%';
        }
        else {
          document.getElementById('progressNumber').innerHTML = 'unable to compute';
        }
      }

      function uploadComplete(evt) {
        /* This event is raised when the server send back a response */
	alert(evt.target.responseText);
      }

      function uploadFailed(evt) {
        alert("There was an error attempting to upload the file.");
      }

      function uploadCanceled(evt) {
        alert("The upload has been canceled by the user or the browser dropped the connection.");
      }
