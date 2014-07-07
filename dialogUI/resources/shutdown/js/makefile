com_dir = ../../common/js
com_coffee = ${com_dir}/utils.coffee ${com_dir}/config.coffee ${com_dir}/animation.coffee ${com_dir}/module.coffee ${com_dir}/widget.coffee 

common_coffee = ${com_coffee} powerdbus.coffee utils.coffee

powerchoose_coffee = ${common_coffee} powerchoose.coffee main.coffee

all: ${powerchoose_coffee}
	coffee -cj shutdown.js ${powerchoose_coffee}
