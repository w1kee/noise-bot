# noise-bot
a discord bot for making noises

## installation
prerequisites:
Go 1.16, POSIX shell, git

- `git clone https://github.com/w1kee/noise-bot`
- `cd noise-bot`
- `mkdir sounds`
- copy a bunch of short mp3s into sounds directory and name them to match regex: `/^[A-Za-z0-9]+$/` + extension
- `go build`
- `echo "YOUR-DISCORD-TOKEN" > token`
- `./noise-bot`
- Enjoy

## use

in discord, you can run `!!help`, and the bot will list all of the sounds you put in.
you can use the sounds by running `!!<soundNameWithoutExtension>`

## license

this project is licensed under the GNU General Public License Version 3.0
