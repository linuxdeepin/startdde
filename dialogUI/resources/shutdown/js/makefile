com_dir = ../../common/js
com_coffee = ${com_dir}/utils.coffee ${com_dir}/config.coffee ${com_dir}/animation.coffee ${com_dir}/module.coffee ${com_dir}/widget.coffee

powerchoose_coffee = background.coffee power_func.coffee utils.coffee confirmdialog.coffee powerchoose.coffee PowerChooseDialog.coffee

shutdown_coffee =  background.coffee power_func.coffee utils.coffee confirmdialog.coffee ShutdownDialog.coffee

logout_coffee =  background.coffee power_func.coffee utils.coffee confirmdialog.coffee LogoutDialog.coffee

reboot_coffee =  background.coffee power_func.coffee utils.coffee confirmdialog.coffee RebootDialog.coffee

all: ${com_coffee} ${powerchoose_coffee} ${shutdown_coffee} ${logout_coffee} ${reboot_coffee}
	coffee -cj powerchoosedialog.js ${com_coffee} ${powerchoose_coffee}
	coffee -cj shutdowndialog.js ${com_coffee} ${shutdown_coffee}
	coffee -cj logoutdialog.js ${com_coffee} ${logout_coffee}
	coffee -cj rebootdialog.js ${com_coffee} ${reboot_coffee}
