use serde::Deserialize;

#[derive(Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NativeConnection {
    #[serde(rename = "baseURL", alias = "baseUrl")]
    pub base_url: String,
    pub session: String,
}

#[cfg(test)]
mod tests {
    use super::NativeConnection;

    #[test]
    fn native_connection_accepts_go_base_url_acronym() {
        let connection: NativeConnection = serde_json::from_str(
            r#"{"baseURL":"http://127.0.0.1:18200","dashboardURL":"http://127.0.0.1:18200/?session=session-token","session":"session-token"}"#,
        )
        .expect("Go native-host JSON should deserialize");
        assert_eq!(connection.base_url, "http://127.0.0.1:18200");
        assert_eq!(connection.session, "session-token");
    }
}
