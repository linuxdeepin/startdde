power_request = (power) ->
    # option = ["lock","suspend","logout","restart","shutdown"]
    document.body.style.cursor = "wait"
    dbus_power = DCore.DBus.session("com.deepin.daemon.ShutdownManager")
    echo "Warning: The system will request ----#{power}----"
    switch power
        when "lock" then dbus_power.RequestLock()
        when "suspend" then dbus_power.RequestSuspend()
        when "logout" then dbus_power.RequestLogout()
        when "restart" then dbus_power.RequestReboot()
        when "shutdown" then dbus_power.RequestShutdown()
        else return

power_can = (power) ->
    result = true
    dbus_power = DCore.DBus.session("com.deepin.daemon.ShutdownManager")
    switch power
        when "lock" then result = true
        when "suspend" then result = dbus_power.CanSuspend_sync()
        when "logout" then result = dbus_power.CanLogout_sync()
        when "restart" then result = dbus_power.CanReboot_sync()
        when "shutdown" then result = dbus_power.CanShutdown_sync()
        else result = false
    echo "power_can : -----------Can_#{power} :#{result}------------"
    if result is undefined then result = true
    return result

power_force = (power) ->
    # option = ["lock","suspend","logout","restart","shutdown"]
    document.body.style.cursor = "wait"
    echo "Warning: The system will ----#{power}---- Force!!"
    dbus_power = DCore.DBus.session("com.deepin.daemon.ShutdownManager")
    switch power
        when "lock" then dbus_power.RequestLock()
        when "suspend" then dbus_power.RequestSuspend()
        when "logout" then dbus_power.ForceLogout()
        when "restart" then dbus_power.ForceReboot()
        when "shutdown" then dbus_power.ForceShutdown()
        else return


