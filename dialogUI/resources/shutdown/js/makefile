com_dir = ../../common/js
com_coffee = ${com_dir}/utils.coffee ${com_dir}/config.coffee ${com_dir}/animation.coffee ${com_dir}/module.coffee ${com_dir}/widget.coffee 

common_coffee = ${com_coffee} power_func.coffee background.coffee utils.coffee confirmdialog.coffee

powerchoose_coffee = ${common_coffee} powerchoose.coffee PowerChooseDialog.coffee

shutdown_coffee = ${common_coffee} ShutdownDialog.coffee

logout_coffee = ${common_coffee} LogoutDialog.coffee

reboot_coffee = ${common_coffee} RebootDialog.coffee

all: ${powerchoose_coffee} ${shutdown_coffee} ${logout_coffee} ${reboot_coffee}
	coffee -cj powerchoosedialog.js ${powerchoose_coffee}
	coffee -cj shutdowndialog.js ${shutdown_coffee}
	coffee -cj logoutdialog.js ${logout_coffee}
	coffee -cj rebootdialog.js ${reboot_coffee}
