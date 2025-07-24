#!/usr/bin/env python3
"""
Simple test script for AI Framework Integration
Tests the core functionality without requiring the full backend
"""

import sys
import os
import json
from datetime import datetime

# Add the backend directory to the path
sys.path.append(os.path.dirname(__file__))

def test_mock_framework():
    """Test the mock AI framework directly"""
    print("🧪 Testing Mock AI Framework")
    print("=" * 50)
    
    try:
        from api.routes.ai_framework import MockAIFramework, AIFrameworkRequest, ConfigAgentRequest
        
        # Create framework instance
        framework = MockAIFramework()
        print("✅ MockAIFramework created successfully")
        
        # Test initialization
        settings = {
            "model": "gemini-2.5-flash-lite",
            "google_application_credentials": "test-credentials"
        }
        init_result = framework.initialize_with_settings(settings)
        print(f"✅ Framework initialization: {init_result}")
        
        # Test agent status
        status = framework.get_agent_status()
        print(f"✅ Agent status: {status.model_dump()}")
        
        # Test supervisor agent
        supervisor_request = AIFrameworkRequest(
            message="Hello, I need help with database migration",
            session_id="test_session",
            migration_id=1,
            agent_type="supervisor"
        )
        supervisor_response = framework.process_message(supervisor_request)
        print(f"✅ Supervisor response: {supervisor_response.agent_response}")
        
        # Test config routing through supervisor
        config_request = AIFrameworkRequest(
            message="I need to create a configuration file",
            session_id="test_session",
            migration_id=1,
            agent_type="supervisor"  # Supervisor handles routing
        )
        config_response = framework.process_message(config_request)
        print(f"✅ Config routing response: {config_response.agent_response}")
        
        # Test schema routing through supervisor
        schema_request = AIFrameworkRequest(
            message="I need help with schema migration",
            session_id="test_session",
            migration_id=1,
            agent_type="supervisor"
        )
        schema_response = framework.process_message(schema_request)
        print(f"✅ Schema routing response: {schema_response.agent_response}")
        
        print("\n🎉 All mock framework tests passed!")
        return True
        
    except Exception as e:
        print(f"❌ Mock framework test failed: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_settings_integration():
    """Test settings integration"""
    print("\n🧪 Testing Settings Integration")
    print("=" * 50)
    
    try:
        from api.routes.settings import load_settings, save_settings
        
        # Test loading settings
        settings = load_settings()
        print(f"✅ Settings loaded: {json.dumps(settings, indent=2)}")
        
        # Test saving settings
        test_settings = {
            "model": "gemini-2.5-flash-lite",
            "google_application_credentials": "test-credentials"
        }
        save_settings(test_settings)
        print("✅ Settings saved successfully")
        
        # Test loading again
        loaded_settings = load_settings()
        print(f"✅ Settings reloaded: {json.dumps(loaded_settings, indent=2)}")
        
        print("🎉 All settings tests passed!")
        return True
        
    except Exception as e:
        print(f"❌ Settings test failed: {e}")
        import traceback
        traceback.print_exc()
        return False

async def test_models():
    """Test model definitions"""
    print("\n🧪 Testing Model Definitions")
    print("=" * 50)
    
    try:
        from api.routes.settings import get_available_models
        
        models = await get_available_models()
        print(f"✅ Available models: {json.dumps(models, indent=2)}")
        
        # Check that Vertex AI models are first
        model_ids = [model["id"] for model in models["models"]]
        print(f"✅ Model order: {model_ids}")
        
        # Verify Vertex AI models are first
        if model_ids[0].startswith("gemini"):
            print("✅ Vertex AI models are first in the list")
        else:
            print("❌ Vertex AI models are not first in the list")
            return False
        
        print("🎉 All model tests passed!")
        return True
        
    except Exception as e:
        print(f"❌ Model test failed: {e}")
        import traceback
        traceback.print_exc()
        return False

async def main():
    """Run all tests"""
    print("🚀 Starting Simple AI Framework Integration Tests")
    print("=" * 60)
    
    tests = [
        ("Mock Framework", test_mock_framework),
        ("Settings Integration", test_settings_integration),
        ("Model Definitions", test_models),
    ]
    
    results = []
    for test_name, test_func in tests:
        try:
            if test_name == "Model Definitions":
                result = await test_func()
            else:
                result = test_func()
            results.append((test_name, result))
        except Exception as e:
            print(f"❌ {test_name} failed with exception: {e}")
            results.append((test_name, False))
    
    # Print summary
    print("\n" + "=" * 60)
    print("📊 Test Results Summary")
    print("=" * 60)
    
    passed = 0
    total = len(results)
    
    for test_name, result in results:
        status = "✅ PASS" if result else "❌ FAIL"
        print(f"{status} - {test_name}")
        if result:
            passed += 1
    
    print(f"\n🎯 Overall: {passed}/{total} tests passed")
    
    if passed == total:
        print("🎉 All tests passed! AI Framework integration is ready.")
        print("\n📋 Next Steps:")
        print("1. Start the backend server: python3 main.py")
        print("2. Run the full integration test: python3 test_ai_framework_integration.py")
        print("3. Test the frontend integration")
    else:
        print("⚠️  Some tests failed. Please check the implementation.")
    
    return passed == total

if __name__ == "__main__":
    import asyncio
    success = asyncio.run(main())
    sys.exit(0 if success else 1) 
