complete -c easy-lima -n '__fish_use_subcommand' -a 'init up start stop restart code ssh exec list inspect doctor logs forward environment rebind adopt delete profile operations repair state config storage version'
complete -c easy-lima -n '__fish_seen_subcommand_from profile' -a 'list show check create import edit apply assign unassign migrate-agent unlock lock forget remove'
complete -c easy-lima -n '__fish_seen_subcommand_from operations' -a 'list inspect resume'
complete -c easy-lima -n '__fish_seen_subcommand_from environment' -a update
complete -c easy-lima -n '__fish_seen_subcommand_from state' -a 'backup restore migrate checkpoint inventory'
complete -c easy-lima -n '__fish_seen_subcommand_from config' -a explain
complete -c easy-lima -n '__fish_seen_subcommand_from storage' -a inspect
complete -c easy-lima -l help -d 'Show command help'
complete -c easy-lima -l json -d 'Versioned metadata JSON'
complete -c easy-lima -l dry-run -d 'Preview effects'
complete -c easy-lima -l yes -d 'Accept the displayed scope'
complete -c easy-lima -l vm -r -d 'VM ID'
complete -c easy-lima -l profile -r -d 'Profile label'
