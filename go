#!/bin/bash

# build and deploy a new stack onto $CYC_CONFIG system

shopt -s expand_aliases
source ~/.bashrc

doBedrock=0
doControlpath=0
doStack=0
doDeploy=0
doTag=0
doPreserve=0
doInfo=0
force=0
isBedrockSetup=0
isContorlpathSetup=0
doVacuum=0
doCC=0
doDeployCore=0
doCPSmoke=0
doDMSmoke=0
doVGSmoke=0

SSH_CMD=./ssh_core_a.sh

# last one listed is used ...
VIP=10.207.80.185	# hop
VIP=100.85.108.104	# ndc
VIP=100.85.108.105	# ndc

function usage() {
    echo "Usage:"
    echo "  $(basename $0) [-h|-i|-a|-b|-B|-c|-f|-s|-d|-t|-p|-v|-C|-D|-S|-M|-V]"
    echo "Options:"
    echo "  -h | --help         This help message"
    echo "  -i | --info         Print bedrock/controlpath/stack and target info"
    echo "  -a | --all          -b -c -s -t -d"
    echo "  -b | --bedrock      Build bedrock into the controlpath"
    echo "  -B | --NodeB        Target array node is Node B"
    echo "  -c | --controlpath  Build controlpath into the stack"
    echo "  -C | --create_cluster Reinit and create_cluster on the array"
    echo "  -D | --Deploy_core  Deploy cyc_core build"
    echo "  -s | --stack        Build stack"
    echo "  -t | --tag          Tag my controlpath"
    echo "  -d | --deploy_build Deploy my controlpath build"
    echo "  -f | --force        Force the build"
    echo "  -v | --vacuum       Vacuum logs on the array"
    echo "  -S | --cp_smoke     Run CP smoke tests"
    echo "  -M | --dm_smoke     Run DM smoke tests"
    echo "  -V | --vg_smoke     Run VG smoke tests"

}

MVN_OPTS="-DskipTests -Dmaven.javadoc.skip -Dmaven.test.skip "

if [ $# -eq  0 ]
then
    usage;
    exit
fi

while [[ $# -gt 0 ]]
do
    key="$1"

    case $key in
        -h|--help|-\?)
            usage;
            exit 0
            ;;

	-i|--info)
            doInfo=1;
            ;;

        -a|--all)
            doBedrock=1
            doControlpath=1
            doStack=1
            doTag=1
            doDeploy=1
            ;;

        -b|--bedrock)
            doBedrock=1
            ;;

        -B|--NodeB)
	    SSH_CMD=./ssh_core_b.sh
            ;;

        -c|--controlpath)
            doControlpath=1
            ;;

        -C|--create_cluster)
            doCC=1
            ;;

        -d|--deploy_build)
            doDeploy=1
            ;;

        -D|--Deploy_core)
            doDeployCore=1
            ;;

        -f|--force)
            force=1
            ;;

        -s|--stack)
            doStack=1
            ;;

        -t|--tag)
            doTag=1
            ;;

        -p|--preserve)
            doPreserve=1
            ;;

        -v|--vacuum)
            doVacuum=1
            ;;

        -S|--cp_smoke)
            doCPSmoke=1
            ;;

        -M|--dm_smoke)
            doDMSmoke=1
            ;;

        -V|--vg_smoke)
            doVGSmoke=1
            ;;

        *)
            # unknown option
            ;;
    esac

    shift # past argument or value
done

function bBuild() {
	echo "Building bedrock"
	if ! (($isBedrockSetup))
	then
		echo "Control Path is *not* set up to use my Bedrock - skipping build"
		exit 1
	fi

	cd /home/cyc/dev/repos/bedrock
	MVN_OPTS=${MVN_OPTS} ./build/build.sh
	if [ $? -ne 0 ]
	then
		echo "***** Bedrock build failed"
		exit 1
	fi
}

function cpBuild() {
	echo "Building controlpath"
	if ! (($isControlpathSetup))
    	then
		echo "Stack is *not* set up to use my Controlpath - skipping build"
		if [ $force -ne 1 ]
		then
			exit 1
		fi
	fi
	cd /home/cyc/dev/repos/cyclone-controlpath
	MVN_OPTS=${MVN_OPTS} ./build/build.sh
	if [ $? -ne 0 ]
	then
		echo "***** Controlpath build failed"
		exit 1
	fi
}

function sBuild() {
	echo "Building stack"
	cd /home/cyc/dev/repos/stack
	MVN_OPTS=${MVN_OPTS} ./build/build.sh
	if [ $? -ne 0 ]
	then
		echo "***** Stack build failed"
		exit 1
	fi
}

function fTag() {
	cd /home/cyc/dev/repos/cyc_core/cyc_platform/src/package/cyc_helpers
	echo "Tagging my controlpath"
	docker tag afeoscyc-mw.cec.lab.emc.com/controlpath/controlpath afeoscyc-mw.cec.lab.emc.com/controlpath/controlpath:viv
	echo "Pushing my controlpath to artifactory"
	docker push afeoscyc-mw.cec.lab.emc.com/controlpath/controlpath:viv
}

function fDeploy() {
	echo "Deploying my controlpath"
	echo "Preparing to pull my  controlpath from artifactory"
	cd /home/cyc/dev/repos/cyc_core/cyc_platform/src/package/cyc_helpers
	${SSH_CMD} sudo touch /cyc_var/cyc_controlpath/dodockerpull
	if [ $? -ne 0 ]
	then
		echo "***** Docker pull prepare failed"
		exit 1
	fi
	
	echo "Restarting my  controlpath"
	${SSH_CMD} sudo /usr/bin/systemctl restart  cyc_controlpath_control.service
	if [ $? -ne 0 ]
	then
		echo "***** Controlpath restart failed"
		exit 1
	fi
}

function fGatherInfo() {
	answer=`grep -m1 "<bedrock.version>" ~/dev/repos/cyclone-controlpath/pom.xml | grep SNAPSHOT`
	isBedrockSetup=$((1-$?))
	answer=`grep "<controlpath.version>" ~/dev/repos/stack/pom.xml | grep SNAPSHOT`
	isControlpathSetup=$((1-$?))
}

function fVacuum() {
	echo "Vacuuming $CYC_CONFIG"
	cd /home/cyc/dev/repos/cyc_core/cyc_platform/src/package/cyc_helpers
	${SSH_CMD} sudo journalctl --rotate && ${SSH_CMD} sudo journalctl --vacuum-size=10M
}

function fCC() {
	cd /home/cyc/dev/repos/cyc_core/cyc_platform/src/package/cyc_helpers
	base=`basename $CYC_CONFIG`
	target=${base##cyc-cfg.txt.}
	echo "Reiniting and Creating Cluster on $target"
	create_cluster.py -sys ${target}  -vip ${VIP} -vpass Password123!  -reinit -post -y -stdout
	if [ $? -ne 0 ]
	then
		echo "***** Create_cluster failed"
		exit 1
	fi
}

function fdeployCore() {
	echo "Deploying my cyc_core in HCI mode"
	cd /home/cyc/dev/repos/cyc_core/cyc_platform/src/package/cyc_helpers
	base=`basename $CYC_CONFIG`
	echo base $base
	target=${base##cyc-cfg.txt.}
        target=${target%%-BM}
	echo "Deploy cyc_core to ${target}" ##./deploy --deploytype san WX-N6009 or ./deploy  WX-N6009
	./deploy ${target}
	#./deploy --deploytype san ${target}

}

function fCPSmoke() {
	echo "Do CP Smoke"
	if [ "${doCC}" == "1" ]
	then
		echo "take a nap for 5 minutes since we just did a create cluster"
		sleep 300
	fi
	cd /home/cyc/dev/repos/cyclone-controlpath
	tests/start_smoke.py  -suite CP -vip ${VIP}

}
function fDMSmoke() {
	echo "Do DM Smoke"
	if [ "${doCC}" == "1" ]
	then
		echo "take a nap for 5 minutes since we just did a create cluster"
		sleep 300
	fi
	cd /home/cyc/dev/repos/cyclone-controlpath
	tests/start_smoke.py  -suite DM -vip ${VIP}
}
function fVGSmoke() {
	echo "Do VG Smoke"
	if [ "${doCC}" == "1" ]
	then
		echo "take a nap for 5 minutes since we just did a create cluster"
		sleep 300
	fi
	cd /home/cyc/dev/repos/cyclone-controlpath
	tests/start_smoke.py  -suite VG -vip ${VIP}
}


fGatherInfo

if [ "${doInfo}" == "1" ]
then
	if [ "${CYC_CONFIG}" == "" ]
	then
		echo "Set \$CYC_CONFIG Variable"
    		exit 1;
	fi
	#echo "Target is: " `basename ${CYC_CONFIG##*-}`
	echo "Target is: " `basename ${CYC_CONFIG##*cyc-cfg.txt.}`
	if (($isBedrockSetup))
	then
		echo "Control Path is set up to use my Bedrock"
	else
		echo "Control Path is *not* set up to use my Bedrock"
	fi

	if (($isControlpathSetup))
	then
		echo "Stack is set up to use my Controlpath"
	else
		echo "Stack is *not* set up to use my Controlpath"
	fi

    exit 0;
fi

if [ "${doBedrock}" == "1" ]
then
	bBuild;
	echo "Bedrock is built"
fi

if [ "${doControlpath}" == "1" ]
then
	cpBuild;
	echo "Controlpath is built"
fi

if [ "${doStack}" == "1" ]
then
	sBuild;
	echo "Stack is built"
fi

if [ "${doTag}" == "1" ]
then
	fTag;
fi

if [ "${doDeploy}" == "1" ]
then
	fDeploy;
fi

if [ "${doDeployCore}" == "1" ]
then
	fdeployCore;
	echo "TBD"
fi

if [ "${doCC}" == "1" ]
then
	fCC;
	echo "Array is reinited and cluster is created"
fi

if [ "${doCPSmoke}" == "1" ]
then
	fCPSmoke;
fi

if [ "${doDMSmoke}" == "1" ]
then
	fDMSmoke;
fi

if [ "${doVGSmoke}" == "1" ]
then
	fVGSmoke;
fi

if [ "${doVacuum}" == "1" ]
then
	fVacuum;
	echo "Array is vacuumed"
fi

exit 0;
