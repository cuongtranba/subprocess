#!/bin/bash

# Interactive echo script that prints back user input
# Exits when user types 'exit' or 'quit' or sends EOF (Ctrl+D)

while true; do
    # Read user input
    read -r input

    # Check if read failed (EOF/Ctrl+D)
    if [ $? -ne 0 ]; then
        echo ""
        break
    fi

    # Check for exit commands
    if [ "$input" = "exit" ] || [ "$input" = "quit" ]; then
        break
    fi

    # Echo back the input
    echo "$input"
done
