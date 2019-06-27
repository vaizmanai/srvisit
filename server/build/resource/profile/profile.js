	function make(m, arg1, arg2, arg3, arg4){
		var req = getXmlHttp()

		req.onreadystatechange = function() {
		
			if (req.readyState == 4) {
			
				if(req.status == 200) {

					if(m == 'getProfile'){
						var obj = JSON.parse(req.responseText);
						
						document.getElementsByName('email')[0].value = obj.Email;
						document.getElementsByName('abc')[0].value = obj.Pass;
						document.getElementsByName('def')[0].value = obj.Pass;
						document.getElementsByName('capt')[0].value = obj.Capt;
						document.getElementsByName('tel')[0].value = obj.Tel;
						document.getElementsByName('logo')[0].value = obj.Logo;
					}
					
				}else if(req.status == 401){
					document.location = '/';
				}else{
					alert('Что-то пошло не так!');
				}
			}
		}

		if (m == 'getProfile'){
			req.open('GET', '/api?make=profile_get', true)
		}
		
		req.send(null)
	}

	function getProfile() {
		make("getProfile");
	}