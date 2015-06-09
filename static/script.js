(function() {
    $("#invite-form").on("submit", function(event){
        event.preventDefault();
        $('.invite-email').removeClass("input-error");
        
        $("#form-success").hide();
        $("#form-failure").hide();
        
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
            $("#form-failure").show();
            $('.invite-email').addClass("input-error");
            document.getElementById("form-failure").textContent = xhr.responseText;
        });
    });
})();