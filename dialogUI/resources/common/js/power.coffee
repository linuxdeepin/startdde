
class Power
    SessionManager = "com.deepin.SessionManager"
    FREEDESKKTOP_LOGIN1 =
        name:"org.freedesktop.login1",
        path:"/org/freedesktop/login1",
        interface:"org.freedesktop.login1.Manager",

    constructor : ->
        @option = ["shutdown","restart","suspend","logout"]
        @dbus_login1 = null
        @inhibitorsList = []
        @inhibit_power = []
        @inhibit_power_app_msg = []
        @get_login1_dbus()
        #@inhibit_set_for_test()

    get_login1_dbus : ->
        try
            @dbus_login1 = DCore.DBus.sys_object(
                FREEDESKKTOP_LOGIN1.name,
                FREEDESKKTOP_LOGIN1.path,
                FREEDESKKTOP_LOGIN1.interface
            )
        catch e
            echo "dbus_login1 error:#{e}"

    power_request : (power) ->
        if not @dbus_login1? then @get_login1_dbus()
        document.body.style.cursor = "wait" if power isnt "suspend" and power isnt "lock"
        echo "Warning: The system will request ----#{power}----"
        switch power
            when "suspend" then @dbus_login1.Suspend(true)
            when "restart" then @dbus_login1.Reboot(true)
            when "shutdown" then @dbus_login1.PowerOff(true)
            else return

    power_force_sys : (power) ->
        if not @dbus_login1? then @get_login1_dbus()
        document.body.style.cursor = "wait" if power isnt "suspend" and power isnt "lock"
        echo "Warning: The system will force ----#{power}----"
        switch power
            when "suspend" then @dbus_login1.Suspend(false)
            when "restart" then @dbus_login1.Reboot(false)
            when "shutdown" then @dbus_login1.PowerOff(false)
            else return

    power_force_session : (power) ->
        dbus_power = null
        try
            dbus_power = DCore.DBus.session(SessionManager)
        catch e
            echo "dbus_power error:#{e}"
        if not dbus_power? then return
        document.body.style.cursor = "wait" if power isnt "suspend" and power isnt "lock"
        echo "Warning: The system will ----#{power}---- Force!!"
        switch power
            when "lock" then dbus_power.RequestLock()
            when "suspend" then dbus_power.RequestSuspend()
            when "logout" then dbus_power.ForceLogout()
            when "restart" then dbus_power.ForceReboot()
            when "shutdown" then dbus_power.ForceShutdown()
            else return

    power_can_freedesktop : (power) ->
        if not @dbus_login1? then @get_login1_dbus()
        result = true
        switch power
            when "suspend" then result = @dbus_login1.CanSuspend_sync()
            when "restart" then result = @dbus_login1.CanReboot_sync()
            when "shutdown" then result = @dbus_login1.CanPowerOff_sync()
            else result = false
        echo "power_can : -----------Can_#{power} :#{result}------------"
        if result is undefined then result = true
        return result


    power_get_inhibit : ->
        echo "power_get_inhibit"
        if not @dbus_login1? then @get_login1_dbus()
        @inhibitorsList = @dbus_login1?.ListInhibitors_sync()
        for inhibit,i in @inhibitorsList
            if inhibit is undefined or inhibit is null then break
            #inhibit[ 0    1   2   3]
            #       power app msg type
            try
                power = inhibit[0]
                app = inhibit[1]
                msg = inhibit[2]
                type = inhibit[3]
                if type is "block"
                    switch power
                        when "shutdown"
                            power = "shutdown"
                            @inhibit_power.push(power)
                            @inhibit_power_app_msg.push({power:power,app:app,msg:msg})
                        when "handle-suspend-key" and "idle"
                            power = "suspend"
                            @inhibit_power.push(power)
                            @inhibit_power_app_msg.push({power:power,app:app,msg:msg})
                        when "handle-power-key"
                            power = ["restart","shutdown","logout"]
                            for p in power
                                @inhibit_power.push(p)
                                @inhibit_power_app_msg.push({power:p,app:app,msg:msg})
            catch e
                echo "power_get_inhibit error:#{e}"

    power_can : (power) ->
        return !(power in @inhibit_power)

    inhibit_msg: (power) ->
        if @power_can(power) then return null
        return inhibit.msg for inhibit in @inhibit_power_app_msg when inhibit.power is power

    inhibit_set_for_test : ->
        echo "--------inhibit_set_for_test-------"
        if not @dbus_login1? then @get_login1_dbus()
        dsc_update_inhibits = ["shutdown","sleep","idle","handle-power-key","handle-suspend-key","handle-hibernate-key","handle-lid-switch"]
        for power in dsc_update_inhibits
            console.log "power for inhibit:#{power}"
            @dbus_login1.Inhibit(
                power,
                "DeepinSoftCenter",
                "Please wait a moment while system update is being performed... Do not turn off your computer.",
                "block"
            )
