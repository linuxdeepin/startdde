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

class Background
    ACCOUNTS_DAEMON = "com.deepin.daemon.Accounts"
    ACCOUNTS_USER =
        obj: ACCOUNTS_DAEMON
        path: "/com/deepin/daemon/Accounts/User1000"
        interface: "com.deepin.daemon.Accounts.User"
    
    GRAPHIC = "com.deepin.api.Graphic"
    APP = null

    constructor:(@id)->
        APP = @id#APP_NAME for DCore[APP]

        @users_name = []
        @users_id_dbus = []
        @users_name_dbus = []
    
        @getDBus()

    getDBus:->
        try
            @Dbus_Account = DCore.DBus.sys(ACCOUNTS_DAEMON)
            for path in @Dbus_Account.UserList
                ACCOUNTS_USER.path = path
                user_dbus = DCore.DBus.sys_object(
                    ACCOUNTS_USER.obj,
                    ACCOUNTS_USER.path,
                    ACCOUNTS_USER.interface
                )
                @users_name.push(user_dbus.UserName)
                @users_id_dbus[user_dbus.Uid] = user_dbus
                @users_name_dbus[user_dbus.UserName] = user_dbus
        catch e
            echo "Dbus_Account #{ACCOUNTS_DAEMON} ERROR: #{e}"

        try
            @Dbus_Graphic = DCore.DBus.session(GRAPHIC)
        catch e
            echo "#{GRAPHIC} dbus ERROR: #{e}"

    get_user_id:(user)->
        id = null
        try
            id = @users_name_dbus[user].Uid
        catch e
            echo "get_user_id #{e}"
        if not id? then id = "1000"
        return id

    get_user_bg:(uid)->
        bg = null
        try
            bg = @users_id_dbus[uid].BackgroundFile
        catch e
            echo "get_user_bg #{e}"
        return bg

    get_blur_background:(uid)->
        bg = @get_user_bg(uid)
        echo "get_blur_background #{uid},userbg:#{bg}"

        BackgroundBlurPictPath = null
        PATH_MSG = null
        try
            path = @Dbus_Graphic.BackgroundBlurPictPath_sync(bg,"",30.0,1.0)
            switch path[0]
                when -1 then PATH_MSG = "failed"
                when 0 then PATH_MSG = "return_bg"
                when 1 then PATH_MSG = "succeed"
            echo "BackgroundBlurPictPath_sync: #{path[0]}: #{PATH_MSG}-----#{path[1]}-----"
            BackgroundBlurPictPath = path[1]
        catch e
            echo "bg:#{e}"
        BackgroundBlurPictPath = DEFAULT_BG if not BackgroundBlurPictPath?
        echo "BackgroundBlurPictPath final:#{BackgroundBlurPictPath}"
        return BackgroundBlurPictPath
    
    get_default_username:->
        try
            if APP is "Greeter"
                @_default_username = DCore[APP].get_default_user()
            else
                @_default_username = DCore[APP].get_username()
        catch e
            echo "get_default_username:#{e}"
        return @_default_username
    
    get_current_user_background:->
        @_current_username = @get_default_username()
        @_current_userid = @get_user_id(@_current_username)
        return @get_user_bg(uid)
    
    get_current_user_blur_background:->
        @_current_username = @get_default_username()
        @_current_userid = @get_user_id(@_current_username)
        return @get_blur_background(@_current_userid)
