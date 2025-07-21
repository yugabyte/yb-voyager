
from fastapi import FastAPI
from fastapi.responses import RedirectResponse

from langserve import add_routes

from react_agent import agent

# from pydantic import BaseModel
# from typing import List, Union
# from langchain_core.messages import AIMessage, HumanMessage, SystemMessage




# class ChatInputType(BaseModel):
#     messages: List[Union[HumanMessage, AIMessage, SystemMessage]]



app = FastAPI()

@app.get("/")
async def redirect_root_to_docs():
    return RedirectResponse("/docs")


# Edit this to add the chain you want to add
# prebuilt_react_agent_runnable = agent.with_types(input_type=ChatInputType)
add_routes(app, agent, path="/agent")

if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000)