github_org := "example"
os := `uname | tr '[:upper:]' '[:lower:]'`
arch := `arch`

# Install banshee
install: 
    curl -o banshee -L "https://github.com/TheJokersThief/Banshee/releases/latest/download/banshee-{{ os }}-{{ arch }}" \
        && chmod +x banshee \
        && sudo mv banshee /usr/local/bin/

# Create a new migration
new_migration name:
    mkdir {{ current_timestamp }}_{{ name }}
    cd {{ current_timestamp }}_{{ name }} && wget -q https://raw.githubusercontent.com/theJokersThief/Banshee/master/examples/migration_config/migration.yaml
    sed -i '' 's$"examples/prbody.md"$"{{ current_timestamp }}{{ name }}/prbody.md"$' "{{ current_timestamp }}{{ name }}/migration.yaml"
    sed -i '' 's$example-org${{ github_org }}$' "{{ current_timestamp }}_{{ name }}/migration.yaml"
    cd {{ current_timestamp }}_{{ name }} && wget -q https://raw.githubusercontent.com/theJokersThief/Banshee/master/examples/prbody.md
    @# Created new migration: {{ current_timestamp }}_{{ name }}
