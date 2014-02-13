class Background extends Widget
    Dbus_Account = null
    users_path = []
    users_name = []
    users_id = []
    is_greeter = false

    constructor:->
        super
        Dbus_Account = DCore.DBus.sys("org.freedesktop.Accounts")
        try
            DCore.Greeter.get_date()
            is_greeter = true
        catch error
            is_greeter = false



    get_all_users:->
        if is_greeter
            users_name = DCore.Greeter.get_users()
        else
            users_path = Dbus_Account.ListCachedUsers_sync()
            for path in users_path
                user_dbus = DCore.DBus.sys_object("org.freedesktop.Accounts",path,"org.freedesktop.Accounts.User")
                name = user_dbus.UserName
                id = user_dbus.Uid
                users_name.push(name)
                users_id.push(id)
        #echo users_name
        return users_name


    get_default_username:->
        if is_greeter
            _default_username = DCore.Greeter.get_default_user()
        else
            try
                _default_username = DCore.Lock.get_username()
            catch e
                _default_username = DCore.Shutdown.get_username()
        return _default_username

    get_user_id:(user)->
        if users_id.length == 0 or users_name.length == 0 then @get_all_users()
        id = null
        for tmp,j in users_name
            if user is tmp
                id = users_id[j]
        if not id?
            id = users_id[0]
        if not id?
            id = "1000"
        return id

    set_blur_background:(user)->
        #BackgroundBlurPictPath = localStorage.getItem("BackgroundBlurPictPath")
        BackgroundBlurPictPath = null
        if not BackgroundBlurPictPath?
            userid = new String()
            userid = @get_user_id(user)
            echo "current user #{user}'s userid:#{userid}"
            Dbus_Account_deepin = DCore.DBus.sys("com.deepin.api.Image")
            path = Dbus_Account_deepin.BackgroundBlurPictPath_sync(userid.toString(),"")
            if path[0]
                BackgroundBlurPictPath = path[1]
            else
                # here should getPath by other methods!
                BackgroundBlurPictPath = path[1]
        echo "BackgroundBlurPictPath:#{BackgroundBlurPictPath}"
        localStorage.setItem("BackgroundBlurPictPath",BackgroundBlurPictPath)
        if !is_greeter
            try
                document.body.style.backgroundImage = "url(#{BackgroundBlurPictPath})"
            catch e
                echo e
                document.body.style.backgroundImage = "url(/usr/share/backgrounds/default_background.jpg)"

    set_current_user_blur_background:->
        current_user = @get_default_username()
        @set_blur_background(current_user)
