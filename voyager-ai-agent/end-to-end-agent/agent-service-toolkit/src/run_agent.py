import asyncio
from typing import cast
from uuid import uuid4

from dotenv import load_dotenv
from langchain_core.messages import HumanMessage
from langchain_core.runnables import RunnableConfig
from langgraph.graph import MessagesState
from langgraph.graph.state import CompiledStateGraph

load_dotenv()

from agents import DEFAULT_AGENT, get_agent, get_all_agent_info  # noqa: E402

# The default agent uses StateGraph.compile() which returns CompiledStateGraph



async def main() -> None:
    agents = await get_all_agent_info()
    print(f"\nAgents in main: {agents}\n")
    agent = cast(CompiledStateGraph, get_agent("voyager-agent"))
    inputs: MessagesState = {
        "messages": [HumanMessage(input("Enter your message: "))]
    }
    result = await agent.ainvoke(
        input=inputs,
        config=RunnableConfig(configurable={"thread_id": uuid4()}),
    )
    for message in result["messages"]:
        message.pretty_print()
    # result["messages"][-1].pretty_print()

    # Draw the agent graph as png
    # requires:
    # brew install graphviz
    # export CFLAGS="-I $(brew --prefix graphviz)/include"
    # export LDFLAGS="-L $(brew --prefix graphviz)/lib"
    # pip install pygraphviz
    #
    # agent.get_graph().draw_png("agent_diagram.png")


if __name__ == "__main__":
    asyncio.run(main())
