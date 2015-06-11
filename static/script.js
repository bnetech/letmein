navigator.browserInfo = function(){
    var ua= navigator.userAgent, tem,
    M= ua.match(/(opera|chrome|safari|firefox|msie|trident(?=\/))\/?\s*(\d+)/i) || [];
    if(/trident/i.test(M[1])){
        tem=  /\brv[ :]+(\d+)/g.exec(ua) || [];
        return 'IE '+(tem[1] || '');
    }
    if(M[1]=== 'Chrome'){
        tem= ua.match(/\b(OPR|Edge)\/(\d+)/);
        if(tem!= null) return tem.slice(1).join(' ').replace('OPR', 'Opera');
    }
    M= M[2]? [M[1], M[2]]: [navigator.appName, navigator.appVersion, '-?'];
    if((tem= ua.match(/version\/(\d+)/i))!= null) M.splice(1, 1, tem[1]);
    return { 'browser': M[0], 'version': M[1] };
};

(function() {
    var bi = navigator.browserInfo();
    var ub = $("#unsupported-browser");
    console.log(bi.version);
    console.log(bi.browser);
    console.log(parseInt(bi.version) < 10);
    if (bi.browser === "MSIE" && parseInt(bi.version) < 10 ){
        ub.addClass("show");
        document.getElementById("unsupported-browser").textContent = "Oh no! IE" + bi.version + " is not supported by BNEte.ch. Probably time to get a new browser!";
    }

    var pageOpened = new Date();

    $("#invite-form").on("submit", function(event){
        event.preventDefault();

        $('.invite-email').removeClass("input-error");
        
        $("#form-success").hide();
        $("#form-failure").hide();

        if ($('#honeypot').val() !== "") {
            document.getElementById("form-failure").textContent = "Looks like you're a robot.";
            $("#form-loading").hide();
            $("#form-failure").show();
            return
        }

        $('#page-opended').val(pageOpened.toISOString());

        var serialized = $("#invite-form").serialize();
        console.log(serialized);
        $(this).find("input").prop("disabled", "disabled");
        $("#form-loading").show();

        var xhr = $.post("/invite", serialized);
        xhr.done(function(){
            $("#form-success").show();
            $("#form-loading").hide();
        });
        xhr.fail(function(xhr, textStatus, errorThrown){
            $("#invite-form").find("input").prop("disabled", "");
            $("#form-loading").hide();
            document.getElementById("form-failure").textContent = xhr.responseText;
            $("#form-failure").show();
            $('.invite-email').addClass("input-error");
        });
    });
})();