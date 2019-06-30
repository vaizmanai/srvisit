		function checkLang(){
			var lang = gup("lang",window.location.href);
			if (lang == null) {
				if (navigator.language == "ru-RU") {
					window.location="/index_ru.html?lang=ru"
				}else{
					window.location="/index.html?lang=eng"
				}
			}			
		}

		function gup( name, url ) {
			if (!url) url = location.href;
			name = name.replace(/[\[]/,"\\\[").replace(/[\]]/,"\\\]");
			var regexS = "[\\?&]"+name+"=([^&#]*)";
			var regex = new RegExp( regexS );
			var results = regex.exec( url );
			return results == null ? null : results[1];
		}

		function make(m){
			var req = getXmlHttp()
			req.onreadystatechange = function() {
				if (req.readyState == 4) {
					if(req.status == 200) {
						if (m == 'version') {
							resp = JSON.parse(req.responseText);

							document.getElementById("version").innerHTML = resp[0] + " - " + resp[1];
						}
					}else{
						if (m == 'version') {
							document.getElementById("version").innerHTML = "current version";
						}
					}
				}
			}

			if (m == 'version') {
				req.open('GET', 'http://server.rvisit.net:8090/api?make=version', true)
			}
			
			req.send(null)
		}
		
		function getXmlHttp(){
			var xmlhttp;
			try {
				xmlhttp = new ActiveXObject("Msxml2.XMLHTTP");
				} catch (e) {
				try {
					xmlhttp = new ActiveXObject("Microsoft.XMLHTTP");
					} catch (E) {
						xmlhttp = false;
						}
					}
			if (!xmlhttp && typeof XMLHttpRequest!='undefined') {
				xmlhttp = new XMLHttpRequest();
				}
			return xmlhttp;
		}