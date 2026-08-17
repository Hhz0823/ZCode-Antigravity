use serde::Deserialize;
use std::collections::BTreeMap;

#[derive(Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NativeConnection {
    #[serde(rename = "baseURL", alias = "baseUrl")]
    pub base_url: String,
    pub session: String,
}

#[derive(Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConnectorResponse {
    pub model: String,
    #[serde(rename = "baseURL", alias = "baseUrl")]
    pub base_url: String,
    pub connectors: Vec<AgentConnector>,
}

#[derive(Clone, Default, Deserialize)]
pub struct AgentConnector {
    pub name: String,
    pub description: String,
    pub snippets: BTreeMap<String, String>,
}

#[cfg(test)]
mod tests {
    use super::{ConnectorResponse, NativeConnection};

    #[test]
    fn native_connection_accepts_go_base_url_acronym() {
        let connection: NativeConnection = serde_json::from_str(
            r#"{"baseURL":"http://127.0.0.1:18200","dashboardURL":"http://127.0.0.1:18200/?session=session-token","session":"session-token"}"#,
        )
        .expect("Go native-host JSON should deserialize");
        assert_eq!(connection.base_url, "http://127.0.0.1:18200");
        assert_eq!(connection.session, "session-token");
    }

    #[test]
    fn connector_response_accepts_go_base_url_acronym() {
        let response: ConnectorResponse = serde_json::from_str(
            r#"{"provider":"antigravity","model":"gemini-3.7-flash","baseURL":"http://127.0.0.1:18080","connectors":[]}"#,
        )
        .expect("Go connector JSON should deserialize");
        assert_eq!(response.base_url, "http://127.0.0.1:18080");
        assert_eq!(response.model, "gemini-3.7-flash");
    }
}
