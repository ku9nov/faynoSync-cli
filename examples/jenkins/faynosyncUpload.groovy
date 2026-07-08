def call(Map config) {
    assert config.app     : 'faynosyncUpload: "app" is required'
    assert config.version : 'faynosyncUpload: "version" is required'
    assert config.channel : 'faynosyncUpload: "channel" is required'

    String app           = config.app
    def fileArg          = (config.file instanceof List) ?
                               config.file.collect { "--file ${it}" }.join(' ') :
                               "--file ${config.get('file', "./${app}")}"
    String version       = config.version
    String channel       = config.channel
    String platform      = config.get('platform', 'linux')
    String arch          = config.get('arch', 'amd64')
    String credentialsId = config.get('credentialsId', "faynosync-token-${app}")
    String changelog     = config.get('changelog', '')
    boolean publish      = config.get('publish', false) as boolean
    boolean critical     = config.get('critical', false) as boolean
    boolean intermediate = config.get('intermediate', false) as boolean
    String updater       = config.get('updater', '')
    String signature     = config.get('signature', '')

    List<String> extraFlags = []
    if (publish)      { extraFlags << '--publish' }
    if (critical)     { extraFlags << '--critical' }
    if (intermediate) { extraFlags << '--intermediate' }
    if (updater)      { extraFlags << "--updater ${updater}" }
    if (signature)    { extraFlags << "--signature ${signature}" }

    writeFile file: 'CUSTOM_CHANGELOG.md', text: changelog

    withCredentials([string(credentialsId: credentialsId, variable: 'FAYNOSYNC_TOKEN')]) {
        def response = sh(
            script: """
                faynosync-cli upload \
                    --app ${app} \
                    ${fileArg} \
                    --version ${version} \
                    --channel ${channel} \
                    ${extraFlags.join(' ')} \
                    --platform ${platform} \
                    --arch ${arch} \
                    --changelog-file CUSTOM_CHANGELOG.md
            """,
            returnStdout: true
        ).trim()

        echo "CLI response: ${response}"
        sh 'rm -f CUSTOM_CHANGELOG.md'

        if (!response.contains('Upload completed')) {
            unstable("Upload failed: 'Upload completed' not found in response.")
        } else {
            echo "Upload successful. Response contains 'Upload completed'."
        }

        return response
    }
}
