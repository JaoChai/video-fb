import os
import pytest
from unittest.mock import patch


def test_config_loads_from_env():
    env = {
        "ANTHROPIC_API_KEY": "sk-ant-test",
        "KIE_API_KEY": "kie-test",
        "ELEVENLABS_VOICE": "Brian",
        "ELEVENLABS_STABILITY": "0.8",
        "OUTPUT_DIR": "/tmp/output",
        "ASSETS_DIR": "/tmp/assets",
    }
    with patch.dict(os.environ, env, clear=False):
        from src.config import load_config

        cfg = load_config()
        assert cfg.anthropic_api_key == "sk-ant-test"
        assert cfg.kie_api_key == "kie-test"
        assert cfg.elevenlabs_voice == "Brian"
        assert cfg.elevenlabs_stability == 0.8
        assert cfg.output_dir == "/tmp/output"


def test_config_defaults():
    env = {
        "ANTHROPIC_API_KEY": "sk-ant-test",
        "KIE_API_KEY": "kie-test",
    }
    with patch.dict(os.environ, env, clear=False):
        from src.config import load_config

        cfg = load_config()
        assert cfg.elevenlabs_voice == "Adam"
        assert cfg.elevenlabs_stability == 0.5
