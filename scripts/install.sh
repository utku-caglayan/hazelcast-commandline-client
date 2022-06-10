#!/usr/bin/env bash

PROGRAM_NAME="hzc"
HZCLI_HOME="$HOME/.local/share/hz-cli"

ghExtractTag() {
  tagUrl=$(curl "https://github.com/$1/releases/latest" -s -L -I -o /dev/null -w '%{url_effective}')
  printf "%s\n" "${tagUrl##*tag/v}"
}

bin_id=""
machine=$(uname -m)

case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    "linux")
        case "$machine" in
            "arm64"*) bin_id='Linux_arm64' ;;
            *"x86_64") bin_id='Linux_x86_64' ;;
        esac
    ;;
    "darwin")
        case "$machine" in
            *"arm64") bin_id='Darwin_arm64' ;;
            *"x86_64") bin_id='Darwin_x86_64' ;;
        esac
    ;;
esac

tag=$(ghExtractTag "utku-caglayan/hazelcast-commandline-client")
releaseUrl=$(printf "https://github.com/utku-caglayan/hazelcast-commandline-client/releases/download/v%s/hazelcast-commandline-client_%s_%s.tar.gz" "$tag" "$tag" "$bin_id")

mkdir -p $HOME/$PROGRAM_NAME && chmod +x $HOME/$PROGRAM_NAME
curl -L --silent "$releaseUrl" | tar -xz -C "$HOME/$PROGRAM_NAME"

mkdir -p $HOME/.local/bin
mv $HOME/$PROGRAM_NAME/$PROGRAM_NAME $HOME/.local/bin
echo "Hazelcast Commandline Client (CLC) is downloaded to \$HOME/.local/bin/$PROGRAM_NAME"
echo

read -rd '' addToPathDirectivesZSH << EOF
* Add \$HOME/.local/bin to PATH to access hzc from any directory.
  Append the line below to add it to .zshrc:
  export PATH=\$HOME/.local/bin:\$PATH

EOF

read -rd '' addToPathDirectivesBASH << EOF
* Add \$HOME/.local/bin to PATH to access hzc from any directory.
  Append the line below to add it to .bashrc:
  export PATH=\$HOME/.local/bin:\$PATH

EOF

read -rd '' zshAutocompletionDirectives << EOF
* If you have not already enabled autocompletion for Zsh, append the line below to ~/.zshrc
  autoload -U compinit; compinit

* Enable autocompletion for Hazelcast Commandline Client (CLC).
  Create a symbolic link of autocompletion script to one of your paths in your fpath by executing:
  sudo ln -s $HZCLI_HOME/autocompletion/zsh/hzc \${fpath[1]}/_hzc

* Restart your terminal for the CLC autocompletion to take effect or renew your session via:
  /bin/zsh

EOF

read -rd '' bashAutocompletionDirectives << EOF

* Restart your terminal for the CLC autocompletion to take effect or renew your session via:
  exec "\$BASH"

EOF

echo "Installation for Zsh:"
if [[ ! -r $HOME/.zshrc || ! "$(cat $HOME/.zshrc)" == *"$(echo "export PATH=$HOME/.local/bin:$PATH")"* ]]; then
    echo "$addToPathDirectivesZSH"
    echo
fi
mkdir -p $HZCLI_HOME/autocompletion/zsh
mv "${HOME}/${PROGRAM_NAME}/extras/zsh_completion.zsh" "${HZCLI_HOME}/autocompletion/zsh/${PROGRAM_NAME}"
echo "$zshAutocompletionDirectives"

echo
echo "Installation for Bash:"
if [[ ! $PATH == *"$HOME/.local/bin"* ]]; then
    echo "$addToPathDirectivesBASH"
    echo
fi
xdg_home="$XDG_DATA_HOME"
if [ -z "$xdg_home" ]; then
    # XDG_DATA_HOME was not set
    xdg_home="$HOME/.local/share"
fi
bash_completion_dir="$BASH_COMPLETION_USER_DIR"
if [ -z "$bash_completion_dir" ]; then
    # BASH_COMPLETION_USER_DIR was not set
    bash_completion_dir="$xdg_home/bash-completion"
fi
mkdir -p "${bash_completion_dir}/completions"
mkdir -p "${HZCLI_HOME}/autocompletion/bash"
mv "${HOME}/${PROGRAM_NAME}/extras/bash_completion.sh" "${HZCLI_HOME}/autocompletion/bash/hz-cli"
ln -s $HZCLI_HOME/autocompletion/bash/hz-cli "${bash_completion_dir}/completions/$PROGRAM_NAME"
echo "$bashAutocompletionDirectives"

mkdir -p "${HZCLI_HOME}/bin/"
mv "${HOME}/${PROGRAM_NAME}/scripts/uninstall.sh" "${HZCLI_HOME}/bin"
chmod +x ${HZCLI_HOME}/bin/uninstall.sh
rm -rf "${HOME}/${PROGRAM_NAME}"
echo "You can uninstall hz command line tools by running ${HZCLI_HOME}/bin/uninstall.sh"
echo "Example usage: hzc --help"