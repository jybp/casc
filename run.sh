export CASC_APP=$1
export CASC_REGION="eu"
export CASC_CDN="eu"

if [ "$CASC_APP" == "w3" ]; then
	export CASC_DIR="/Applications/Warcraft III"
	export CASC_PATTERN="War3.mpq:Movies/*.avi"
elif [ "$CASC_APP" == "d3" ]; then
	export CASC_DIR="/Applications/Diablo III"
	export CASC_PATTERN="enUS/Data_D3/Locale/enUS/Cutscenes/Cinematic_1*.ogv"
elif [ "$CASC_APP" == "s1" ]; then
	export CASC_DIR="/Applications/StarCraft"
	export CASC_PATTERN="HD2/Smk/*.webm"
fi

make $2