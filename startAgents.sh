
echo "number of agents: $2"
echo "name $1 "

if [ "$#" -ne 2 ]; then
    echo "Usage: {name}, # of agents"
    exit 1
fi



for ((i=1; i<=$2; i++))
do
    echo "starting.."
    go run . -name="$1" -isRand=true & 
    FOO_PID=$!
done

wait
echo END
