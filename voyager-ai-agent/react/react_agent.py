
from langgraph.prebuilt import create_react_agent

from llm import llm_ollama_mistral

def get_weather(city: str) -> str:  
    """Get weather for a given city."""
    return f"It's sunny in {city}!"

agent = create_react_agent(
    model=llm_ollama_mistral,
    tools=[get_weather],  
    prompt="You are a helpful assistant"  
)

# Run the agent
if __name__ == "__main__":
    output = agent.invoke(
        {"messages": [{"role": "user", "content": "what is the weather in sf"}]}
    )

    for m in output["messages"]:
        m.pretty_print()


# API
# app = FastAPI(
#     title="LangChain Server",
#     version="1.0",
#     description="Spin up a simple api server using LangChain's Runnable interfaces",
# )
# # Adds routes to the app for using the chain under:
# # /invoke
# # /batch
# # /stream
# # /stream_events
# add_routes(
#     app,
#     agent,
# )

# if __name__ == "__main__":
#     import uvicorn

#     uvicorn.run(app, host="localhost", port=8000)




