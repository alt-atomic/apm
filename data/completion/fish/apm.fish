# fish completion for apm

function __fish_apm_perform_completion
    set -l args (commandline -opc)
    set -l current (commandline -ct)
    set -l results ($args[1] $args[2..-1] $current --generate-shell-completion 2>/dev/null)

    for line in $results[-1..1]
        if test (string trim -- $line) = ""
            set results $results[1..-2]
        else
            break
        end
    end

    for line in $results
        if not string match -q -- "apm*" $line
            set -l parts (string split -m 1 ":" -- "$line")
            if test (count $parts) -eq 2
                printf "%s\t%s\n" "$parts[1]" "$parts[2]"
            else
                printf "%s\n" "$line"
            end
        end
    end
end

complete -c apm -e
complete -c apm -f -a '(__fish_apm_perform_completion)'
