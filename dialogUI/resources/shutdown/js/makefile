com_dir = ../../common/js
com_coffee = ${com_dir}/utils.coffee ${com_dir}/config.coffee ${com_dir}/animation.coffee ${com_dir}/module.coffee ${com_dir}/widget.coffee ${com_dir}/power.coffee ${com_dir}/accounts.coffee

powerchoose_coffee = ${com_coffee} utils.coffee  powerchoose.coffee main.coffee

all: ${powerchoose_coffee}
	coffee -cj shutdown.js $^
