# Load with: source /path/to/easy-lima.bash
_easy_lima() {
    local current="${COMP_WORDS[COMP_CWORD]}" previous="${COMP_WORDS[COMP_CWORD-1]}" words value
    case "$previous" in
        easy-lima) words='init up start stop restart code ssh exec list inspect doctor logs forward environment rebind adopt delete profile operations repair state config storage version' ;;
        profile) words='list show check create import edit apply assign unassign migrate-agent unlock lock forget remove' ;;
        operations) words='list inspect resume' ;;
        environment) words='update' ;;
        state) words='backup restore migrate checkpoint inventory' ;;
        inventory) words='refresh' ;;
        config) words='explain' ;;
        storage) words='inspect' ;;
        *) words='' ;;
    esac
    if [[ "$current" == --* ]]; then
        words='--help --json --dry-run --yes --vm --profile --no-profile --cpus --memory --disk --template --presets --apply --no-start --start --config --project-config --limactl --capture-output'
    fi
    COMPREPLY=()
    if [[ -n "$words" ]]; then
        while IFS= read -r value; do COMPREPLY+=("$value"); done < <(compgen -W "$words" -- "$current")
    else
        while IFS= read -r value; do COMPREPLY+=("$value"); done < <(compgen -d -- "$current")
    fi
}
complete -o filenames -F _easy_lima easy-lima
