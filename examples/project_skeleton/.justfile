
os := `uname | tr '[:upper:]' '[:lower:]'`
arch := `arch`

install: 
    curl -o banshee -L "https://github.com/TheJokersThief/Banshee/releases/latest/download/banshee-{{ os }}-{{ arch }}" \
        && chmod +x banshee \
        && sudo mv banshee /usr/local/bin/