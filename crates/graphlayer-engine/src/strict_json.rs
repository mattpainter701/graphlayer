//! Preserve exact numeric tokens while rejecting duplicate nested object keys.
use serde::de::{self, MapAccess, Visitor};
use serde::{Deserialize, Deserializer};
use serde_json::{value::RawValue, Map, Value};
use std::{collections::HashSet, fmt};

pub struct StrictValue(pub Value);
impl<'de> Deserialize<'de> for StrictValue {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let raw = Box::<RawValue>::deserialize(deserializer)?;
        parse(raw.get(), 0).map(Self).map_err(de::Error::custom)
    }
}
fn parse(raw: &str, depth: usize) -> Result<Value, serde_json::Error> {
    if depth > 128 { return Err(de::Error::custom("JSON nesting limit")); }
    match raw.trim_start().as_bytes().first() {
        Some(b'{') => {
            struct ObjectVisitor;
            impl<'de> Visitor<'de> for ObjectVisitor {
                type Value = Vec<(String, Box<RawValue>)>;
                fn expecting(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result { f.write_str("an unambiguous object") }
                fn visit_map<A: MapAccess<'de>>(self, mut object: A) -> Result<Self::Value, A::Error> {
                    let mut seen = HashSet::new();
                    let mut fields = Vec::new();
                    while let Some(key) = object.next_key::<String>()? {
                        if !seen.insert(key.clone()) { return Err(de::Error::custom("duplicate JSON field")); }
                        fields.push((key, object.next_value::<Box<RawValue>>()?));
                    }
                    Ok(fields)
                }
            }
            let mut decoder = serde_json::Deserializer::from_str(raw);
            let fields = decoder.deserialize_map(ObjectVisitor)?;
            decoder.end()?;
            let mut object = Map::new();
            for (key, value) in fields { object.insert(key, parse(value.get(), depth + 1)?); }
            Ok(Value::Object(object))
        }
        Some(b'[') => {
            let values: Vec<Box<RawValue>> = serde_json::from_str(raw)?;
            values.into_iter().map(|value| parse(value.get(), depth + 1)).collect::<Result<Vec<_>, _>>().map(Value::Array)
        }
        _ => {
            let value: Value = serde_json::from_str(raw)?;
            if let Value::Number(number) = &value {
                if raw.contains(['.', 'e', 'E']) && !number.as_f64().is_some_and(f64::is_finite) {
                    return Err(de::Error::custom("invalid number"));
                }
            }
            Ok(value)
        }
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn duplicate_nested_fields_are_rejected() {
        for raw in [r#"{"config":{"secret":1,"secret":2}}"#, r#"[{"x":1,"x":2}]"#, r#"{"x":1} {}"#, r#"{"x":1e999}"#] {
            assert!(serde_json::from_str::<StrictValue>(raw).is_err());
        }
        assert!(serde_json::from_str::<StrictValue>(r#"{"x":[true,null,2,"text"]}"#).is_ok());
    }
    #[test]
    fn numbers_and_reserved_names_retain_their_actual_json_types() {
        let parsed = serde_json::from_str::<StrictValue>(r#"{"big":100000000000000000000000000000000000001,"small":1e-5,"negative":-0.0,"literal":{"$serde_json::private::Number":"123"}}"#).unwrap().0;
        assert_eq!(parsed["big"].to_string(), "100000000000000000000000000000000000001");
        assert_eq!(parsed["small"].as_f64(), Some(1e-5));
        assert!(parsed["negative"].as_f64().unwrap().is_sign_negative());
        assert!(parsed["literal"].is_object());
    }
}
