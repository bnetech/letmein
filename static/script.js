(function() {

    var pageOpened = new Date();

    $("#invite-form").on("submit", function(event){
        event.preventDefault();

        $('.invite-email').removeClass("input-error");
        
        $("#form-success").hide();
        $("#form-failure").hide();

        if ($('#honeypot').val() !== "") {
            document.getElementById("form-failure").textContent = "Looks like you're a robot :(";
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