# /usr/bin/env bash


_completion_loader()
{
	# you can use COMP_LINE, COMP_POINT, COMP_KEY, and COMP_TYPE
	# $1 is command, $2 is completion phrase $3 is word prior (which can be command)
	
	case ${COMP_CWORD} in
        1)
			TOPLEVEL=$( circleci | sed -e '1,/Available Commands:/d'  | sed '/^$/q' | awk '{print $1}' | tr '\n' ' ' )
			COMPREPLY=$(compgen -W "$TOPLEVEL" $2)
			;;
		2)
			NEXTLEVEL=$( circleci $3 | sed -e '1,/Available Commands:/d'  | sed '/^$/q' | awk '{print $1}' | tr '\n' ' ' )
			COMPREPLY=$(compgen -W "$NEXTLEVEL" $2)
			;;
		*)
			COMPREPLY=()
	esac	


	# store suggestions in COMPREPLY

}

complete circleci #make sure the command itself autocompletes.
complete -F _completion_loader circleci  # auto completion for arguments

