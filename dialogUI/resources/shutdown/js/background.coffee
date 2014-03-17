#Copyright (c) 2011 ~ 2014 Deepin, Inc.
#              2011 ~ 2014 bluth
#
#encoding: utf-8
#Author:      bluth <yuanchenglu@linuxdeepin.com>
#Maintainer:  bluth <yuanchenglu@linuxdeepin.com>
#
#This program is free software; you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation; either version 3 of the License, or
#(at your option) any later version.
#
#This program is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.
#
#You should have received a copy of the GNU General Public License
#along with this program; if not, see <http://www.gnu.org/licenses/>.

class Background extends Widget
    Dbus_Account = null
    user_info = []
    users_path = []
    users_name = []
    users_id = []
    users_bg = []
    
    DAEMON_ACCOUNTS = "com.deepin.daemon.Accounts"
    ACCOUNTS_USER =
        obj: DAEMON_ACCOUNTS
        path: "/com/deepin/daemon/Accounts/User1000"
        interface: "com.deepin.daemon.Accounts.User"
    DEFAULT_BG = "/usr/share/backgrounds/default_background.jpg"
    API_GRAPH = "com.deepin.api.Graphic"
    
    constructor:->
        super
        Dbus_Account = DCore.DBus.sys(DAEMON_ACCOUNTS)


    get_all_users:->
        if is_greeter
            users_name = DCore.Greeter.get_users()
        else
            users_path = Dbus_Account.UserList
            for path in users_path
                ACCOUNTS_USER.path = path
                user_dbus = DCore.DBus.sys_object(
                    ACCOUNTS_USER.obj,
                    ACCOUNTS_USER.path,
                    ACCOUNTS_USER.interface
                )
                name = user_dbus.UserName
                id = user_dbus.Uid
                bg = user_dbus.BackgroundFile
                if not bg? then bg = DEFAULT_BG
                users_name.push(name)
                users_id.push(id)
                users_bg.push(bg)
                
                user = user_dbus
                info =
                    Uid:user.Uid,
                    UserName:user.UserName,
                    BackgroundFile:user.BackgroundFile,
                    IconFile:user.IconFile,
                    LoginTime:user.LoginTime,
                    BgBlur:null

                user_info[user.Uid] = info
        
        echo user_info[users_id[0]]
        echo user_info.length
        echo users_name
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
    
    get_user_bg:(user)->
        if users_bg.length == 0 or users_name.length == 0 then @get_all_users()
        bg = null
        for tmp,j in users_name
            if user is tmp
                bg = users_bg[j]
        if not bg?
            bg = DEFAULT_BG
        return bg

    set_blur_background:(user)->
        #BackgroundBlurPictPath = localStorage.getItem("BackgroundBlurPictPath")
        BackgroundBlurPictPath = null
        if not BackgroundBlurPictPath?
            userid = new String()
            userid = @get_user_id(user)
            userbg= @get_user_bg(user)
            echo "current user #{user},userid:#{userid},userbg:#{userbg}"
            try
                dbus = DCore.DBus.session(API_GRAPH)
                path = dbus.BackgroundBlurPictPath_sync(userbg,"",30,1)
                echo "--------------------------"
                echo path
                if path[0]
                    echo "BackgroundBlurPictPath_sync:true,#{path[1]}--"
                    BackgroundBlurPictPath = path[1]
                else
                    echo "BackgroundBlurPictPath_sync:false,#{path[1]}--"
                    # here should getPath by other methods!
                    BackgroundBlurPictPath = path[1]
                    BackgroundBlurPictPath = DEFAULT_BG if not BackgroundBlurPictPath?
            catch error
                echo "bg:" + error
                BackgroundBlurPictPath = DEFAULT_BG
                echo "BackgroundBlurPictPath:#{BackgroundBlurPictPath}"
        user_info[userid].BgBlur = BackgroundBlurPictPath
        localStorage.setObject("user_info",user_info)
        localStorage.setItem("BackgroundBlurPictPath",BackgroundBlurPictPath)
        if !is_greeter
            try
                document.body.style.backgroundImage = "url(#{BackgroundBlurPictPath})"
            catch e
                echo e
                document.body.style.backgroundImage = "url(#{DEFAULT_BG})"

    set_current_user_blur_background:->
        current_user = @get_default_username()
        @set_blur_background(current_user)
